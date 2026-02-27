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
	t.Logf("Setting up cluster: istio=%t, fips=%t, experimental=%t, prerequisites=%t, helmValues=%v, chart=%s",
		cfg.InstallIstio, cfg.OperateInFIPSMode, cfg.EnableExperimental, cfg.DeployPrerequisites, cfg.HelmValues, cfg.ChartPath)

	// Ensure Istio is in the desired state
	if err := ensureIstioState(t, k8sClient, cfg); err != nil {
		return fmt.Errorf("failed to ensure Istio state: %w", err)
	}

	// Deploy/upgrade manager with desired configuration
	if err := ensureManagerDeployed(t, k8sClient, cfg); err != nil {
		return fmt.Errorf("failed to ensure manager deployed: %w", err)
	}

	// Deploy test prerequisites if enabled
	if err := ensureTestPrerequisites(t, k8sClient, cfg.DeployPrerequisites); err != nil {
		return fmt.Errorf("failed to ensure test prerequisites: %w", err)
	}

	t.Log("Cluster setup complete")

	return nil
}

// ensureTestPrerequisites deploys test prerequisites if enabled.
// Server-side apply is idempotent, so this is safe to call multiple times.
func ensureTestPrerequisites(t TestingT, k8sClient client.Client, deploy bool) error {
	if !deploy {
		t.Log("Skipping test prerequisites deployment")
		return nil
	}

	return deployTestPrerequisites(t, k8sClient)
}

// ensureManagerDeployed ensures the telemetry manager is deployed with the desired configuration.
// It handles experimental mode changes which require uninstall before reinstall due to CRD conflicts.
// After any configuration change (FIPS mode, helm values, etc.), it waits for the manager to be stable.
func ensureManagerDeployed(t TestingT, k8sClient client.Client, cfg Config) error {
	ctx := t.Context()

	// Detect current configuration to know if we're making changes
	currentExperimental := detectExperimentalEnabled(ctx)
	currentFIPS := detectFIPSEnabled(ctx)
	configChanged := false

	// Check if experimental mode change requires uninstall first
	// Switching between experimental and default subcharts requires uninstall
	// because both subcharts contain CRD templates that conflict
	if currentExperimental != cfg.EnableExperimental && releaseExists(ctx) {
		if cfg.SkipManagerRemoval {
			return fmt.Errorf("experimental mode change required (%t -> %t) but SkipManagerRemoval is set", currentExperimental, cfg.EnableExperimental)
		}

		t.Logf("Experimental mode change detected (%t -> %t), removing manager first...", currentExperimental, cfg.EnableExperimental)

		if err := undeployManager(t, k8sClient); err != nil {
			return fmt.Errorf("failed to remove manager for experimental mode change: %w", err)
		}

		if err := waitForCRDsDeletion(t, k8sClient); err != nil {
			t.Logf("Warning: failed waiting for CRDs deletion: %v", err)
		}

		configChanged = true
	}

	// Track FIPS mode change
	if currentFIPS != cfg.OperateInFIPSMode && releaseExists(ctx) {
		t.Logf("FIPS mode change detected (%t -> %t)", currentFIPS, cfg.OperateInFIPSMode)

		configChanged = true
	}

	// Deploy/upgrade manager (helm upgrade --install is idempotent)
	if err := deployManager(t, k8sClient, cfg); err != nil {
		return fmt.Errorf("failed to deploy manager: %w", err)
	}

	// Wait for stability after configuration changes or for upgrade scenarios
	if configChanged || cfg.SkipManagerRemoval {
		t.Log("Waiting for deployment rollout to complete...")

		if err := waitForRolloutComplete(ctx, k8sClient, t, 3*time.Minute); err != nil {
			return fmt.Errorf("rollout did not complete: %w", err)
		}

		if err := waitForSinglePod(ctx, k8sClient, t, 1*time.Minute); err != nil {
			return fmt.Errorf("multiple pods still running: %w", err)
		}

		t.Logf("Waiting %s for manager to reconcile resources...", reconcileDelay)
		time.Sleep(reconcileDelay)
	}

	return nil
}

// ensureIstioState ensures Istio is in the desired state (installed or not installed).
// It handles cleanup of problematic states and triggers install/uninstall as needed.
func ensureIstioState(t TestingT, k8sClient client.Client, cfg Config) error {
	ctx := t.Context()

	// Check current Istio state
	istioState := DetectIstioState(ctx, k8sClient)
	t.Logf("Detected Istio state: %s", istioState)

	// Handle problematic Istio states that need cleanup before proceeding
	if istioState.NeedsReinstall() {
		if err := handleIstioCleanup(t, k8sClient, istioState); err != nil {
			return fmt.Errorf("failed to cleanup Istio: %w", err)
		}
		// After cleanup, Istio is not installed
		istioState = IstioNotInstalled
	}

	// Determine if Istio state matches desired state
	currentInstalled := istioState == IstioFullyInstalled

	// Handle Istio changes (requires special ordering)
	if currentInstalled != cfg.InstallIstio {
		if cfg.SkipManagerRemoval {
			return fmt.Errorf("istio state change required (%t -> %t) but SkipManagerRemoval is set", currentInstalled, cfg.InstallIstio)
		}

		if err := handleIstioChange(t, k8sClient, currentInstalled, cfg.InstallIstio); err != nil {
			return err
		}
	}

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

// handleIstioCleanup handles cleanup for problematic Istio states (orphaned or partially installed).
func handleIstioCleanup(t TestingT, k8sClient client.Client, state IstioState) error {
	switch state {
	case IstioOrphaned:
		t.Log("Cleaning up orphaned Istio CR (no manager running)...")

		if err := removeIstioCRFinalizers(t, k8sClient); err != nil {
			return fmt.Errorf("failed to remove Istio CR finalizers: %w", err)
		}

	case IstioPartiallyInstalled:
		t.Log("Cleaning up partial Istio installation...")

		if err := uninstallIstio(t, k8sClient); err != nil {
			return fmt.Errorf("failed to uninstall partial Istio: %w", err)
		}

	case IstioNotInstalled, IstioFullyInstalled:
		// No cleanup needed for these states
	}

	t.Log("Istio cleanup complete")

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
