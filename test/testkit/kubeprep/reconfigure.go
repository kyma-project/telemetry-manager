package kubeprep

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SetupCluster configures the cluster for test execution.
// It always runs helm upgrade --install (idempotent) and deploys prerequisites.
// Only Istio changes require special handling (manager must be removed first).
//
// This function is idempotent and safe to call multiple times.
func SetupCluster(t TestingT, k8sClient client.Client, cfg Config) error {
	ctx := t.Context()

	t.Logf("Setting up cluster: istio=%t, fips=%t, experimental=%t, helmValues=%v, chart=%s",
		cfg.InstallIstio, cfg.OperateInFIPSMode, cfg.EnableExperimental, cfg.HelmValues, cfg.ChartPath)

	// Check current Istio state - only detection needed
	currentIstioInstalled := DetectIstioInstalled(ctx, k8sClient)

	// Check for orphaned Istio CR (CR exists but no Istio manager running)
	// This can happen when the Istio manager was removed while the CR had a finalizer
	if DetectOrphanedIstioCR(ctx, k8sClient) {
		t.Log("Detected orphaned Istio CR (no manager running), removing finalizers...")

		if err := removeIstioCRFinalizers(t, k8sClient); err != nil {
			t.Logf("Warning: failed to remove Istio CR finalizers: %v", err)
		}
	}

	// Check for partial Istio installation (CR exists but istiod not ready)
	// This can happen if a previous installation attempt failed/timed out
	if cfg.InstallIstio && DetectIstioPartiallyInstalled(ctx, k8sClient) {
		t.Log("Detected partial Istio installation, cleaning up before retry...")

		if err := cleanupPartialIstioInstallation(t, k8sClient); err != nil {
			t.Logf("Warning: failed to cleanup partial Istio installation: %v", err)
		}

		// After cleanup, Istio is definitely not installed
		currentIstioInstalled = false
	}

	// Handle Istio changes (requires special ordering)
	if currentIstioInstalled != cfg.InstallIstio {
		if err := handleIstioChange(t, k8sClient, currentIstioInstalled, cfg.InstallIstio); err != nil {
			return fmt.Errorf("failed to handle Istio change: %w", err)
		}
	}

	// Check current experimental state and handle changes
	// Switching between experimental and default subcharts requires uninstall first
	// because both subcharts contain CRD templates that conflict
	currentExperimental := detectExperimentalEnabled(ctx)
	if currentExperimental != cfg.EnableExperimental && releaseExists(ctx) {
		t.Logf("Experimental mode change detected (%t -> %t), removing manager first...", currentExperimental, cfg.EnableExperimental)

		if err := undeployManager(t, k8sClient); err != nil {
			return fmt.Errorf("failed to remove manager for experimental mode change: %w", err)
		}
		// Wait for CRDs to be fully deleted before reinstalling
		if err := waitForCRDsDeletion(t, k8sClient); err != nil {
			t.Logf("Warning: failed waiting for CRDs deletion: %v", err)
		}
	}

	// Deploy/upgrade manager (helm upgrade --install is idempotent)
	if err := deployManager(t, k8sClient, cfg); err != nil {
		return fmt.Errorf("failed to deploy manager: %w", err)
	}

	// Deploy prerequisites (server-side apply is idempotent)
	if err := deployTestPrerequisites(t, k8sClient); err != nil {
		return fmt.Errorf("failed to deploy prerequisites: %w", err)
	}

	t.Log("Cluster setup complete")

	return nil
}

// handleIstioChange handles Istio installation/uninstallation.
// The manager must be removed before Istio changes to avoid webhook/sidecar conflicts.
func handleIstioChange(t TestingT, k8sClient client.Client, currentInstalled, desiredInstalled bool) error {
	// Remove manager FIRST (prevents conflicts during Istio changes)
	t.Log("Removing manager before Istio change...")

	if err := undeployManager(t, k8sClient); err != nil {
		return fmt.Errorf("failed to remove manager: %w", err)
	}

	// Wait for CRDs to be fully deleted
	if err := waitForCRDsDeletion(t, k8sClient); err != nil {
		return fmt.Errorf("failed waiting for CRDs deletion: %w", err)
	}

	// Perform Istio change
	if !currentInstalled && desiredInstalled {
		t.Log("Installing Istio...")
		installIstio(t, k8sClient)
	} else if currentInstalled && !desiredInstalled {
		t.Log("Uninstalling Istio...")

		if err := uninstallIstio(t, k8sClient); err != nil {
			return fmt.Errorf("failed to uninstall Istio: %w", err)
		}
	}

	return nil
}

// waitForCRDsDeletion waits for telemetry CRDs to be fully deleted
func waitForCRDsDeletion(t TestingT, k8sClient client.Client) error {
	telemetryCRDs := []string{
		"logpipelines.telemetry.kyma-project.io",
		"tracepipelines.telemetry.kyma-project.io",
		"metricpipelines.telemetry.kyma-project.io",
		"telemetries.operator.kyma-project.io",
		"logparsers.telemetry.kyma-project.io",
	}

	timeout := 2 * time.Minute
	interval := 2 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		allDeleted := true

		for _, crdName := range telemetryCRDs {
			exists, err := crdExists(t.Context(), k8sClient, crdName)
			if err != nil {
				t.Logf("Warning: error checking CRD %s: %v", crdName, err)
				continue
			}

			if exists {
				allDeleted = false
				break
			}
		}

		if allDeleted {
			return nil
		}

		time.Sleep(interval)
	}

	return fmt.Errorf("timeout waiting for CRDs to be deleted")
}

// cleanupPartialIstioInstallation removes a partial Istio installation.
// This is called when the Istio CR exists but istiod is not ready,
// indicating a failed previous installation attempt.
func cleanupPartialIstioInstallation(t TestingT, k8sClient client.Client) error {
	t.Log("Cleaning up partial Istio installation...")

	// Use the existing uninstallIstio function which handles proper cleanup order
	if err := uninstallIstio(t, k8sClient); err != nil {
		return fmt.Errorf("failed to uninstall partial Istio: %w", err)
	}

	t.Log("Partial Istio installation cleaned up")

	return nil
}

// removeIstioCRFinalizers removes finalizers from the Istio CR to allow deletion.
// This is needed when the Istio manager is not running and cannot clean up the finalizers itself.
func removeIstioCRFinalizers(t TestingT, k8sClient client.Client) error {
	ctx := t.Context()

	istioCR := &unstructured.Unstructured{}
	istioCR.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.kyma-project.io",
		Version: "v1alpha2",
		Kind:    "Istio",
	})

	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      "default",
		Namespace: "kyma-system",
	}, istioCR)
	if err != nil {
		return fmt.Errorf("failed to get Istio CR: %w", err)
	}

	finalizers := istioCR.GetFinalizers()
	if len(finalizers) == 0 {
		t.Log("Istio CR has no finalizers")
		return nil
	}

	t.Logf("Removing finalizers from Istio CR: %v", finalizers)

	// Remove all finalizers
	istioCR.SetFinalizers(nil)

	if err := k8sClient.Update(ctx, istioCR); err != nil {
		return fmt.Errorf("failed to remove finalizers from Istio CR: %w", err)
	}

	t.Log("Finalizers removed from Istio CR")

	return nil
}
