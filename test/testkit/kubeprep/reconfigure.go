package kubeprep

import (
	"context"
	"fmt"
	"sort"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReconfigureCluster applies minimal changes to bring cluster from current to desired state.
// It intelligently calculates the diff between current and desired configurations and
// applies only the necessary changes.
//
// Reconfiguration strategy:
//   - For Istio changes: Full uninstall/reinstall cycle (manager conflicts with Istio changes)
//   - For other changes (FIPS, experimental, HelmValues): Just helm upgrade --install
//
// This function is idempotent and safe to call multiple times with the same config.
func ReconfigureCluster(t TestingT, k8sClient client.Client, current, desired Config) error {
	t.Logf("Reconfiguring cluster: current=%+v -> desired=%+v", current, desired)

	// Calculate what changed
	diff := calculateDiff(current, desired)

	// Istio changes require full uninstall/reinstall cycle
	if diff.NeedsIstioChange {
		return reconfigureWithIstioChange(t, k8sClient, current, desired)
	}

	// For non-Istio changes, just use helm upgrade --install
	if diff.NeedsManagerRedeploy && !desired.SkipManagerDeployment {
		t.Log("Updating manager configuration via helm upgrade...")

		if err := deployManager(t, k8sClient, desired); err != nil {
			return fmt.Errorf("failed to update manager: %w", err)
		}
	}

	// Handle prerequisites:
	// - If NeedsReinstall was set (customized deployment detected), ensure prerequisites exist
	// - If prerequisites update is explicitly needed
	needsPrerequisites := (current.NeedsReinstall && !desired.SkipManagerDeployment && !desired.SkipPrerequisites) ||
		diff.NeedsPrerequisitesUpdate
	if needsPrerequisites {
		if err := updatePrerequisites(t, k8sClient, desired); err != nil {
			return fmt.Errorf("failed to update prerequisites: %w", err)
		}
	}

	if diff.NeedsManagerRedeploy || diff.NeedsPrerequisitesUpdate {
		t.Log("Cluster reconfiguration complete")
	}

	return nil
}

// reconfigureWithIstioChange handles the full uninstall/reinstall cycle needed for Istio changes.
// The manager must be removed before Istio changes to avoid webhook/sidecar conflicts.
func reconfigureWithIstioChange(t TestingT, k8sClient client.Client, current, desired Config) error {
	// Step 1: Remove manager FIRST (prevents conflicts during Istio changes)
	if !current.SkipManagerDeployment {
		t.Log("Removing manager before Istio reconfiguration...")

		if err := undeployManager(t, k8sClient, current); err != nil {
			return fmt.Errorf("failed to remove manager: %w", err)
		}

		// Wait for CRDs to be fully deleted
		if err := waitForCRDsDeletion(t, k8sClient); err != nil {
			return fmt.Errorf("failed waiting for CRDs deletion: %w", err)
		}
	}

	// Step 2: Istio changes (now safe - manager is removed)
	if err := reconfigureIstio(t, k8sClient, current.InstallIstio, desired.InstallIstio); err != nil {
		return fmt.Errorf("failed to reconfigure Istio: %w", err)
	}

	// Step 3: Reinstall manager with new configuration
	if !desired.SkipManagerDeployment {
		t.Log("Reinstalling manager after Istio reconfiguration...")

		if err := deployManager(t, k8sClient, desired); err != nil {
			return fmt.Errorf("failed to reinstall manager: %w", err)
		}
	}

	// Step 4: Prerequisites - needed after manager reinstall (Telemetry CR was deleted with CRDs)
	if !desired.SkipManagerDeployment && !desired.SkipPrerequisites {
		if err := updatePrerequisites(t, k8sClient, desired); err != nil {
			return fmt.Errorf("failed to update prerequisites: %w", err)
		}
	}

	t.Log("Cluster reconfiguration with Istio change complete")

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
		// - Current state is unknown or customized (NeedsReinstall flag set)
		// - Any manager-affecting config differs
		// - HelmValues changed (any customization triggers redeploy)
		NeedsManagerRedeploy: current.NeedsReinstall ||
			current.OperateInFIPSMode != desired.OperateInFIPSMode ||
			current.EnableExperimental != desired.EnableExperimental ||
			!helmValuesEqual(current.HelmValues, desired.HelmValues) ||
			current.SkipManagerDeployment != desired.SkipManagerDeployment,

		// Prerequisites update needed if skip prerequisites changes
		NeedsPrerequisitesUpdate: current.SkipPrerequisites != desired.SkipPrerequisites,
	}
}

// helmValuesEqual compares two slices of helm values
func helmValuesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	// Sort and compare for order-independent comparison
	aSorted := make([]string, len(a))
	bSorted := make([]string, len(b))

	copy(aSorted, a)
	copy(bSorted, b)
	sort.Strings(aSorted)
	sort.Strings(bSorted)

	for i := range aSorted {
		if aSorted[i] != bSorted[i] {
			return false
		}
	}

	return true
}

// reconfigureIstio handles Istio installation or uninstallation
func reconfigureIstio(t TestingT, k8sClient client.Client, currentInstalled, desiredInstalled bool) error {
	if !currentInstalled && desiredInstalled {
		t.Log("Installing Istio...")
		installIstio(t, k8sClient) // Fails test on error via require.NoError

		return nil
	}

	if currentInstalled && !desiredInstalled {
		t.Log("Uninstalling Istio...")
		return uninstallIstio(t, k8sClient)
	}

	return nil
}

// updatePrerequisites updates test prerequisites (Telemetry CR, network policies, etc.)
func updatePrerequisites(t TestingT, k8sClient client.Client, desired Config) error {
	if desired.SkipPrerequisites {
		return cleanupPrerequisites(t, k8sClient)
	}

	if desired.SkipManagerDeployment {
		return nil // Can't deploy prerequisites without CRDs
	}

	return deployTestPrerequisites(t, k8sClient)
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
