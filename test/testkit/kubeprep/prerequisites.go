package kubeprep

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
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

	// allowSelfMonitorAPIServerEgressNetworkPolicyYAML is a format string for a NetworkPolicy that allows
	// the selfmonitor Prometheus pod to reach the Kubernetes API server for kubernetes SD target discovery.
	// The production NetworkPolicy only lists pod-specific egress rules, which implicitly blocks all other
	// egress (including to the API server) when the deny-all policy is active.
	// %s is substituted with the API server cluster IP (e.g. "10.43.0.1/32").
	allowSelfMonitorAPIServerEgressNetworkPolicyYAML = `---
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
    to:
    - ipBlock:
        cidr: %s/32
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
		_, isK3d := detectK3DCluster(ctx)
		if isK3d == nil {
			apiServerIP, err := getAPIServerClusterIP(ctx, k8sClient)
			if err != nil {
				return fmt.Errorf("failed to get API server cluster IP: %w", err)
			}

			yaml := fmt.Sprintf(allowSelfMonitorAPIServerEgressNetworkPolicyYAML, apiServerIP)
			if err := applyYAML(ctx, k8sClient, yaml); err != nil {
				return fmt.Errorf("failed to apply self-monitor API server egress network policy: %w", err)
			}
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
		_, isK3d := detectK3DCluster(ctx)
		if isK3d == nil {
			apiServerIP, err := getAPIServerClusterIP(ctx, k8sClient)
			if err != nil {
				return fmt.Errorf("failed to get API server cluster IP: %w", err)
			}

			yaml := fmt.Sprintf(allowSelfMonitorAPIServerEgressNetworkPolicyYAML, apiServerIP)
			if err := deleteYAML(ctx, k8sClient, yaml); err != nil {
				return fmt.Errorf("failed to delete self-monitor API server egress network policy: %w", err)
			}
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

// getAPIServerClusterIP returns the cluster IP of the kubernetes service in the default namespace,
// which is the virtual IP used to reach the Kubernetes API server from within the cluster.
func getAPIServerClusterIP(ctx context.Context, k8sClient client.Client) (string, error) {
	var svc corev1.Service
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: "kubernetes", Namespace: "default"}, &svc); err != nil {
		return "", fmt.Errorf("failed to get kubernetes service: %w", err)
	}

	if svc.Spec.ClusterIP == "" {
		return "", fmt.Errorf("kubernetes service has no cluster IP")
	}

	return svc.Spec.ClusterIP, nil
}
