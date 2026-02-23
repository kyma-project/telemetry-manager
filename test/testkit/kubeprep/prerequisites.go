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
	ctx := t.Context()

	t.Log("Deploying test prerequisites...")

	if err := applyYAML(ctx, k8sClient, telemetryCRYAML); err != nil {
		return fmt.Errorf("failed to apply Telemetry CR: %w", err)
	}

	if err := applyYAML(ctx, k8sClient, networkPolicyYAML); err != nil {
		return fmt.Errorf("failed to apply network policy: %w", err)
	}

	if err := applyYAML(ctx, k8sClient, shootInfoConfigMapYAML); err != nil {
		return fmt.Errorf("failed to apply shoot-info ConfigMap: %w", err)
	}

	return nil
}

// DeployTestPrerequisitesPublic is a public wrapper for deployTestPrerequisites.
// It deploys test fixtures required for e2e tests (Telemetry CR, network policy, shoot-info ConfigMap).
// Must be called AFTER manager deployment (needs Telemetry CRD).
func DeployTestPrerequisitesPublic(t TestingT, k8sClient client.Client) error {
	return deployTestPrerequisites(t, k8sClient)
}
