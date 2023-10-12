package kyma

import (
	"k8s.io/apimachinery/pkg/types"
)

const (
	DefaultNamespaceName = "default"
	SystemNamespaceName  = "kyma-system"

	MetricGatewayBaseName = "telemetry-metric-gateway"
	MetricAgentBaseName   = "telemetry-metric-agent"
	TraceGatewayBaseName  = "telemetry-trace-collector"
	DefaultTelemetryName  = "default"
)

var (
	TelemetryOperatorWebhookServiceName = types.NamespacedName{Name: "telemetry-operator-webhook", Namespace: SystemNamespaceName}

	MetricGatewayName          = types.NamespacedName{Name: MetricGatewayBaseName, Namespace: SystemNamespaceName}
	MetricGatewayNetworkPolicy = types.NamespacedName{Name: MetricGatewayBaseName + "-pprof-deny-ingress", Namespace: SystemNamespaceName}

	MetricAgentName = types.NamespacedName{Name: MetricAgentBaseName, Namespace: SystemNamespaceName}

	TraceGatewayName          = types.NamespacedName{Name: TraceGatewayBaseName, Namespace: SystemNamespaceName}
	TraceGatewayNetworkPolicy = types.NamespacedName{Name: TraceGatewayBaseName + "-pprof-deny-ingress", Namespace: SystemNamespaceName}

	TelemetryName = types.NamespacedName{Name: DefaultTelemetryName, Namespace: SystemNamespaceName}
)
