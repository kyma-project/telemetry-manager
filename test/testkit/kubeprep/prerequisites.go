package kubeprep

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Embedded fixture YAML files
const (
	telemetryCRYAML = `---
apiVersion: operator.kyma-project.io/v1beta1
kind: Telemetry
metadata:
  name: default
  namespace: kyma-system
`

	networkPolicyYAML = `---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: deny-all-ingress-and-egress
  namespace: kyma-system
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
`

	shootInfoConfigMapYAML = `---
apiVersion: v1
data:
  provider: k3d
  region: europe-west1
  shootName: kyma-telemetry
kind: ConfigMap
metadata:
  labels:
    resources.gardener.cloud/managed-by: gardener
    shoot.gardener.cloud/no-cleanup: "true"
    persistent: "true"
  name: shoot-info
  namespace: kube-system
`
)

// deployTestPrerequisites deploys test fixtures required for e2e tests
// Must be called AFTER manager deployment (needs Telemetry CRD)
func deployTestPrerequisites(t TestingT, k8sClient client.Client) error {
	t.Helper()
	ctx := t.Context()

	t.Log("Deploying test prerequisites...")

	// Apply telemetry CR (requires Telemetry CRD from manager)
	t.Log("Applying default Telemetry CR...")
	if err := applyYAML(ctx, k8sClient, t, telemetryCRYAML); err != nil {
		return fmt.Errorf("failed to apply Telemetry CR: %w", err)
	}

	// Apply network policy
	t.Log("Applying network policy...")
	if err := applyYAML(ctx, k8sClient, t, networkPolicyYAML); err != nil {
		return fmt.Errorf("failed to apply network policy: %w", err)
	}

	// Apply shoot-info ConfigMap
	t.Log("Applying shoot-info ConfigMap...")
	if err := applyYAML(ctx, k8sClient, t, shootInfoConfigMapYAML); err != nil {
		return fmt.Errorf("failed to apply shoot-info ConfigMap: %w", err)
	}

	t.Log("Test prerequisites deployed successfully")
	return nil
}

// cleanupPrerequisites removes test fixtures
func cleanupPrerequisites(t TestingT, k8sClient client.Client) error {
	t.Helper()
	ctx := t.Context()

	t.Log("Cleaning up test prerequisites...")

	// Delete in reverse order (best effort)
	if err := deleteYAML(ctx, k8sClient, shootInfoConfigMapYAML); err != nil {
		t.Logf("Warning: failed to delete shoot-info ConfigMap: %v", err)
	}

	if err := deleteYAML(ctx, k8sClient, networkPolicyYAML); err != nil {
		t.Logf("Warning: failed to delete network policy: %v", err)
	}

	if err := deleteYAML(ctx, k8sClient, telemetryCRYAML); err != nil {
		t.Logf("Warning: failed to delete Telemetry CR: %v", err)
	}

	t.Log("Test prerequisites cleanup complete")
	return nil
}
