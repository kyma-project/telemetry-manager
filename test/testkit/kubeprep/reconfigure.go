package kubeprep

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReconfigureCluster applies minimal changes to bring cluster from current to desired state.
// It intelligently calculates the diff between current and desired configurations and
// applies only the necessary changes in the correct order.
//
// Reconfiguration order (critical for avoiding conflicts):
//  1. Remove manager first (if Istio or manager config changes) - prevents conflicts
//  2. Istio changes (if needed) - install or uninstall
//  3. Reinstall manager (if it was removed) - with new configuration
//  4. Prerequisites (always after manager reinstall, or if explicitly needed)
//
// This function is idempotent and safe to call multiple times with the same config.
func ReconfigureCluster(t TestingT, k8sClient client.Client, current, desired Config) error {
	t.Helper()

	t.Logf("Reconfiguring cluster from current=%+v to desired=%+v", current, desired)

	// Calculate what changed
	diff := calculateDiff(current, desired)

	// Determine if we need to remove and reinstall the manager
	needsManagerReinstall := diff.NeedsIstioChange || diff.NeedsManagerRedeploy

	// Step 1: Remove manager FIRST if Istio or manager config is changing
	// This prevents conflicts when Istio is removed/installed
	if needsManagerReinstall && !current.SkipManagerDeployment {
		t.Log("Removing manager before reconfiguration...")
		if err := undeployManager(t, k8sClient, current); err != nil {
			return fmt.Errorf("failed to remove manager: %w", err)
		}

		// Wait for CRDs to be fully deleted before proceeding
		// This is critical - CRDs take time to terminate and will block new deployments
		t.Log("Waiting for CRDs to be fully deleted...")
		if err := waitForCRDsDeletion(t, k8sClient); err != nil {
			return fmt.Errorf("failed waiting for CRDs deletion: %w", err)
		}
	}

	// Step 2: Istio changes (now safe - manager is removed)
	if diff.NeedsIstioChange {
		if err := reconfigureIstio(t, k8sClient, current.InstallIstio, desired.InstallIstio); err != nil {
			return fmt.Errorf("failed to reconfigure Istio: %w", err)
		}
	}

	// Step 3: Reinstall manager with new configuration (if it was removed)
	if needsManagerReinstall && !desired.SkipManagerDeployment {
		t.Log("Reinstalling manager with new configuration...")
		if err := deployManager(t, k8sClient, desired); err != nil {
			return fmt.Errorf("failed to reinstall manager: %w", err)
		}
	}

	// Step 4: Prerequisites - ALWAYS needed after manager reinstall (Telemetry CR gets deleted with CRDs)
	// Also needed if prerequisites config explicitly changed
	needsPrerequisites := (needsManagerReinstall && !desired.SkipManagerDeployment && !desired.SkipPrerequisites) ||
		diff.NeedsPrerequisitesUpdate
	if needsPrerequisites {
		if err := updatePrerequisites(t, k8sClient, desired); err != nil {
			return fmt.Errorf("failed to update prerequisites: %w", err)
		}
	}

	if !needsManagerReinstall && !diff.NeedsPrerequisitesUpdate {
		t.Log("No reconfiguration needed - cluster already in desired state")
	} else {
		t.Log("Cluster reconfiguration complete")
	}

	return nil
}

// ConfigDiff represents the differences between two cluster configurations
type ConfigDiff struct {
	NeedsIstioChange         bool // Istio installation state changed
	NeedsManagerRedeploy     bool // Manager configuration changed (FIPS, experimental, etc.)
	NeedsPrerequisitesUpdate bool // Prerequisites need to be updated
}

// calculateDiff determines what changes are needed between current and desired config
func calculateDiff(current, desired Config) ConfigDiff {
	return ConfigDiff{
		// Istio change needed if installation state differs
		NeedsIstioChange: current.InstallIstio != desired.InstallIstio,

		// Manager redeploy needed if:
		// - Current state is unknown (NeedsReinstall flag set)
		// - Any manager-affecting config differs
		NeedsManagerRedeploy: current.NeedsReinstall ||
			current.OperateInFIPSMode != desired.OperateInFIPSMode ||
			current.EnableExperimental != desired.EnableExperimental ||
			current.CustomLabelsAnnotations != desired.CustomLabelsAnnotations ||
			current.SkipManagerDeployment != desired.SkipManagerDeployment,

		// Prerequisites update needed if custom labels/annotations change
		// (Telemetry CR might need updates)
		NeedsPrerequisitesUpdate: (current.CustomLabelsAnnotations != desired.CustomLabelsAnnotations ||
			current.SkipPrerequisites != desired.SkipPrerequisites),
	}
}

// reconfigureIstio handles Istio installation or uninstallation
func reconfigureIstio(t TestingT, k8sClient client.Client, currentInstalled, desiredInstalled bool) error {
	t.Helper()

	if !currentInstalled && desiredInstalled {
		// Install Istio
		t.Log("Installing Istio (requirement change: false -> true)...")
		return installIstio(t, k8sClient)
	}

	if currentInstalled && !desiredInstalled {
		// Uninstall Istio
		t.Log("Uninstalling Istio (requirement change: true -> false)...")
		return uninstallIstio(t, k8sClient)
	}

	// No change needed
	return nil
}

// updatePrerequisites updates test prerequisites (Telemetry CR, network policies, etc.)
func updatePrerequisites(t TestingT, k8sClient client.Client, desired Config) error {
	t.Helper()

	t.Log("Updating test prerequisites...")

	// If prerequisites should be skipped, clean them up
	if desired.SkipPrerequisites {
		t.Log("Removing test prerequisites (SKIP_PREREQUISITES=true)...")
		return cleanupPrerequisites(t, k8sClient)
	}

	// If manager is not deployed, we can't deploy prerequisites (needs CRDs)
	if desired.SkipManagerDeployment {
		t.Log("Skipping prerequisites update (manager not deployed, CRDs unavailable)")
		return nil
	}

	// Deploy/update prerequisites
	t.Log("Deploying/updating test prerequisites...")
	return deployTestPrerequisites(t, k8sClient)
}

// waitForCRDsDeletion waits for telemetry CRDs to be fully deleted
func waitForCRDsDeletion(t TestingT, k8sClient client.Client) error {
	t.Helper()

	telemetryCRDs := []string{
		"logpipelines.telemetry.kyma-project.io",
		"tracepipelines.telemetry.kyma-project.io",
		"metricpipelines.telemetry.kyma-project.io",
		"telemetries.operator.kyma-project.io",
		// Experimental CRDs
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
				t.Logf("Waiting for CRD %s to be deleted...", crdName)
				break
			}
		}

		if allDeleted {
			t.Log("All telemetry CRDs deleted")
			return nil
		}

		time.Sleep(interval)
	}

	return fmt.Errorf("timeout waiting for CRDs to be deleted")
}

// crdExists checks if a CRD exists in the cluster
func crdExists(ctx context.Context, k8sClient client.Client, name string) (bool, error) {
	crd := &unstructured.Unstructured{}
	crd.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apiextensions.k8s.io",
		Version: "v1",
		Kind:    "CustomResourceDefinition",
	})

	err := k8sClient.Get(ctx, types.NamespacedName{Name: name}, crd)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
