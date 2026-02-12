package kubeprep

import (
	"context"
	"fmt"
	"log"

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
func deployTestPrerequisites(ctx context.Context, k8sClient client.Client) error {
	log.Println("Deploying test prerequisites...")

	// Apply telemetry CR (requires Telemetry CRD from manager)
	log.Println("Applying default Telemetry CR...")
	if err := applyYAML(ctx, k8sClient, telemetryCRYAML); err != nil {
		return fmt.Errorf("failed to apply Telemetry CR: %w", err)
	}

	// Apply network policy
	log.Println("Applying network policy...")
	if err := applyYAML(ctx, k8sClient, networkPolicyYAML); err != nil {
		return fmt.Errorf("failed to apply network policy: %w", err)
	}

	// Apply shoot-info ConfigMap
	log.Println("Applying shoot-info ConfigMap...")
	if err := applyYAML(ctx, k8sClient, shootInfoConfigMapYAML); err != nil {
		return fmt.Errorf("failed to apply shoot-info ConfigMap: %w", err)
	}

	log.Println("Test prerequisites deployed successfully")
	return nil
}

// cleanupPrerequisites removes test fixtures
func cleanupPrerequisites(ctx context.Context, k8sClient client.Client) error {
	log.Println("Cleaning up test prerequisites...")

	// Delete in reverse order (best effort)
	if err := deleteYAML(ctx, k8sClient, shootInfoConfigMapYAML); err != nil {
		log.Printf("Warning: failed to delete shoot-info ConfigMap: %v", err)
	}

	if err := deleteYAML(ctx, k8sClient, networkPolicyYAML); err != nil {
		log.Printf("Warning: failed to delete network policy: %v", err)
	}

	if err := deleteYAML(ctx, k8sClient, telemetryCRYAML); err != nil {
		log.Printf("Warning: failed to delete Telemetry CR: %v", err)
	}

	log.Println("Test prerequisites cleanup complete")
	return nil
}
