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

	telemetryCRWithGatewayReplicasYAML = `---
apiVersion: operator.kyma-project.io/v1beta1
kind: Telemetry
metadata:
  name: default
  namespace: kyma-system
spec:
  log:
    gateway:
      scaling:
        type: Static
        static:
          replicas: %d
  metric:
    gateway:
      scaling:
        type: Static
        static:
          replicas: %d
  trace:
    gateway:
      scaling:
        type: Static
        static:
          replicas: %d
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

	// allowSelfMonitorAPIServerEgressNetworkPolicy allows the selfmonitor Prometheus pod to reach the
	// Kubernetes API server on port 443 for kubernetes SD target discovery.
	// The production NetworkPolicy only lists pod-specific egress rules, which implicitly blocks
	// all other egress (including to the API server) when the deny-all policy is active.
	allowSelfMonitorAPIServerEgressNetworkPolicy = `---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-self-monitor-apiserver-egress
  namespace: kyma-system
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: telemetry-self-monitor
  policyTypes:
  - Egress
  egress:
  - ports:
    - port: 443
      protocol: TCP
`

	allowFromGardenerVPNShootNetworkPolicy = `---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-from-gardener-apiserver
  namespace: kyma-system
spec:
  podSelector:
    matchLabels:
      kyma-project.io/module: telemetry
  ingress:
    - from:
      - namespaceSelector:
          matchLabels:
            gardener.cloud/purpose: kube-system
        podSelector:
          matchLabels:
            app: vpn-shoot
            resources.gardener.cloud/managed-by: gardener
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
func deployTestPrerequisites(t TestingT, k8sClient client.Client, cfg Config) error {
	ctx := t.Context()

	t.Log("Deploying test prerequisites...")

	telemetryYAML := telemetryCRYAML
	if cfg.GatewayReplicas > 0 {
		telemetryYAML = fmt.Sprintf(telemetryCRWithGatewayReplicasYAML, cfg.GatewayReplicas, cfg.GatewayReplicas, cfg.GatewayReplicas)
	}

	if err := applyYAML(ctx, k8sClient, telemetryYAML); err != nil {
		return fmt.Errorf("failed to apply Telemetry CR: %w", err)
	}

	if err := applyYAML(ctx, k8sClient, networkPolicyYAML); err != nil {
		return fmt.Errorf("failed to apply network policy: %w", err)
	}

	if cfg.AllowSelfMonitorAPIServerEgress {
		if err := applyYAML(ctx, k8sClient, allowSelfMonitorAPIServerEgressNetworkPolicy); err != nil {
			return fmt.Errorf("failed to apply self-monitor API server egress network policy: %w", err)
		}
	}

	if err := applyYAML(ctx, k8sClient, shootInfoConfigMapYAML); err != nil {
		return fmt.Errorf("failed to apply shoot-info ConfigMap: %w", err)
	}

	// Our e2e tests rely on the API server proxy client to assert OTelCollector metrics. Gardener's API server runs as a
	// pod in kube-system, see https://gardener.cloud/docs/gardener/reversed-vpn-tunnel/.
	// This additional network policy enables communication from Gardener's vpn-shoot-server to telemetry components
	// since network policies restrict pod-to-pod communication.
	if err := applyYAML(ctx, k8sClient, allowFromGardenerVPNShootNetworkPolicy); err != nil {
		return fmt.Errorf("failed to apply gardener allow apiserver network policy: %w", err)
	}

	return nil
}

// removeTestPrerequisites removes test fixtures deployed by deployTestPrerequisites.
// This should be called before Istio installation to avoid network policy conflicts.
func removeTestPrerequisites(t TestingT, k8sClient client.Client, cfg Config) error {
	ctx := t.Context()

	t.Log("Removing test prerequisites...")

	if err := deleteYAML(ctx, k8sClient, allowFromGardenerVPNShootNetworkPolicy); err != nil {
		return fmt.Errorf("failed to delete gardener allow apiserver network policy: %w", err)
	}

	if cfg.AllowSelfMonitorAPIServerEgress {
		if err := deleteYAML(ctx, k8sClient, allowSelfMonitorAPIServerEgressNetworkPolicy); err != nil {
			return fmt.Errorf("failed to delete self-monitor API server egress network policy: %w", err)
		}
	}

	if err := deleteYAML(ctx, k8sClient, networkPolicyYAML); err != nil {
		return fmt.Errorf("failed to delete network policy: %w", err)
	}

	if err := deleteYAML(ctx, k8sClient, shootInfoConfigMapYAML); err != nil {
		return fmt.Errorf("failed to delete shoot-info ConfigMap: %w", err)
	}

	if err := deleteYAML(ctx, k8sClient, telemetryCRYAML); err != nil {
		return fmt.Errorf("failed to delete Telemetry CR: %w", err)
	}

	t.Log("Test prerequisites removed")

	return nil
}
