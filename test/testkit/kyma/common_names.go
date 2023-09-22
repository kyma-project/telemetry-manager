package kyma

import (
	"k8s.io/apimachinery/pkg/types"
)

const (
	DefaultNamespaceName    = "default"
	KymaSystemNamespaceName = "kyma-system"

	MetricGatewayBaseName = "telemetry-metric-gateway"
	MetricAgentBaseName   = "telemetry-metric-agent"
	TraceGatewayBaseName  = "telemetry-trace-collector"
)

var (
	MetricGatewayName          = types.NamespacedName{Name: MetricGatewayBaseName, Namespace: KymaSystemNamespaceName}
	MetricGatewayNetworkPolicy = types.NamespacedName{Name: MetricGatewayBaseName + "-pprof-deny-ingress", Namespace: KymaSystemNamespaceName}

	MetricAgentName = types.NamespacedName{Name: MetricAgentBaseName, Namespace: KymaSystemNamespaceName}

	TraceGatewayName          = types.NamespacedName{Name: TraceGatewayBaseName, Namespace: KymaSystemNamespaceName}
	TraceGatewayNetworkPolicy = types.NamespacedName{Name: TraceGatewayBaseName + "-pprof-deny-ingress", Namespace: KymaSystemNamespaceName}
)
