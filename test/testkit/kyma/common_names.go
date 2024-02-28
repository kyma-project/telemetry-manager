package kyma

import (
	"k8s.io/apimachinery/pkg/types"
)

const (
	DefaultNamespaceName     = "default"
	SystemNamespaceName      = "kyma-system"
	KubeNamespace            = "kube-system"
	IstioSystemNamespaceName = "istio-system"

	MetricGatewayBaseName = "telemetry-metric-gateway"
	MetricAgentBaseName   = "telemetry-metric-agent"
	TraceGatewayBaseName  = "telemetry-trace-collector"
	DefaultTelemetryName  = "default"
)

var (
	TelemetryOperatorWebhookServiceName = types.NamespacedName{Name: "telemetry-manager-webhook", Namespace: SystemNamespaceName}

	MetricGatewayName          = types.NamespacedName{Name: MetricGatewayBaseName, Namespace: SystemNamespaceName}
	MetricGatewayMetrics       = types.NamespacedName{Name: MetricGatewayBaseName + "-metrics", Namespace: SystemNamespaceName}
	MetricGatewayNetworkPolicy = types.NamespacedName{Name: MetricGatewayBaseName, Namespace: SystemNamespaceName}
	MetricGatewaySecretName    = types.NamespacedName{Name: MetricGatewayBaseName, Namespace: SystemNamespaceName}

	MetricAgentName          = types.NamespacedName{Name: MetricAgentBaseName, Namespace: SystemNamespaceName}
	MetricAgentMetrics       = types.NamespacedName{Name: MetricAgentBaseName + "-metrics", Namespace: SystemNamespaceName}
	MetricAgentNetworkPolicy = types.NamespacedName{Name: MetricAgentBaseName, Namespace: SystemNamespaceName}

	TraceGatewayName          = types.NamespacedName{Name: TraceGatewayBaseName, Namespace: SystemNamespaceName}
	TraceGatewayMetrics       = types.NamespacedName{Name: TraceGatewayBaseName + "-metrics", Namespace: SystemNamespaceName}
	TraceGatewayNetworkPolicy = types.NamespacedName{Name: TraceGatewayBaseName, Namespace: SystemNamespaceName}
	TraceGatewaySecretName    = types.NamespacedName{Name: TraceGatewayBaseName, Namespace: SystemNamespaceName}

	TelemetryName = types.NamespacedName{Name: DefaultTelemetryName, Namespace: SystemNamespaceName}
)
