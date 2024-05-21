package kyma

import (
	"k8s.io/apimachinery/pkg/types"
)

const (
	DefaultNamespaceName     = "default"
	SystemNamespaceName      = "kyma-system"
	IstioSystemNamespaceName = "istio-system"

	TelemetryManagerMetricsPort = 8080

	MetricGatewayBaseName = "telemetry-metric-gateway"
	MetricAgentBaseName   = "telemetry-metric-agent"
	TraceGatewayBaseName  = "telemetry-trace-collector"
	FluentBitBaseName     = "telemetry-fluent-bit"
	SelfMonitorBaseName   = "telemetry-self-monitor"
	DefaultTelemetryName  = "default"
	WebhookName           = "validation.webhook.telemetry.kyma-project.io"

	MetricGatewayServiceName = "telemetry-otlp-metrics"
	TraceGatewayServiceName  = "telemetry-otlp-traces"
)

var (
	TelemetryManagerMetricsServiceName = types.NamespacedName{Name: "telemetry-manager-metrics", Namespace: SystemNamespaceName}
	TelemetryManagerWebhookServiceName = types.NamespacedName{Name: "telemetry-manager-webhook", Namespace: SystemNamespaceName}

	MetricGatewayName               = types.NamespacedName{Name: MetricGatewayBaseName, Namespace: SystemNamespaceName}
	MetricGatewayMetricsService     = types.NamespacedName{Name: MetricGatewayBaseName + "-metrics", Namespace: SystemNamespaceName}
	MetricGatewayNetworkPolicy      = types.NamespacedName{Name: MetricGatewayBaseName, Namespace: SystemNamespaceName}
	MetricGatewaySecretName         = types.NamespacedName{Name: MetricGatewayBaseName, Namespace: SystemNamespaceName}
	MetricGatewayOTLPService        = types.NamespacedName{Name: MetricGatewayServiceName, Namespace: SystemNamespaceName}
	MetricGatewayServiceAccount     = types.NamespacedName{Name: MetricGatewayBaseName, Namespace: SystemNamespaceName}
	MetricGatewayClusterRole        = types.NamespacedName{Name: MetricGatewayBaseName, Namespace: SystemNamespaceName}
	MetricGatewayClusterRoleBinding = types.NamespacedName{Name: MetricGatewayBaseName, Namespace: SystemNamespaceName}
	MetricGatewayConfigMap          = types.NamespacedName{Name: MetricGatewayBaseName, Namespace: SystemNamespaceName}

	MetricAgentName          = types.NamespacedName{Name: MetricAgentBaseName, Namespace: SystemNamespaceName}
	MetricAgentMetrics       = types.NamespacedName{Name: MetricAgentBaseName + "-metrics", Namespace: SystemNamespaceName}
	MetricAgentNetworkPolicy = types.NamespacedName{Name: MetricAgentBaseName, Namespace: SystemNamespaceName}

	TraceGatewayName               = types.NamespacedName{Name: TraceGatewayBaseName, Namespace: SystemNamespaceName}
	TraceGatewayMetricsService     = types.NamespacedName{Name: TraceGatewayBaseName + "-metrics", Namespace: SystemNamespaceName}
	TraceGatewayNetworkPolicy      = types.NamespacedName{Name: TraceGatewayBaseName, Namespace: SystemNamespaceName}
	TraceGatewaySecretName         = types.NamespacedName{Name: TraceGatewayBaseName, Namespace: SystemNamespaceName}
	TraceGatewayOTLPService        = types.NamespacedName{Name: TraceGatewayServiceName, Namespace: SystemNamespaceName}
	TraceGatewayServiceAccount     = types.NamespacedName{Name: TraceGatewayBaseName, Namespace: SystemNamespaceName}
	TraceGatewayClusterRole        = types.NamespacedName{Name: TraceGatewayBaseName, Namespace: SystemNamespaceName}
	TraceGatewayClusterRoleBinding = types.NamespacedName{Name: TraceGatewayBaseName, Namespace: SystemNamespaceName}
	TraceGatewayConfigMap          = types.NamespacedName{Name: TraceGatewayBaseName, Namespace: SystemNamespaceName}

	FluentBitDaemonSet              = types.NamespacedName{Name: FluentBitBaseName, Namespace: SystemNamespaceName}
	FluentBitServiceAccount         = types.NamespacedName{Name: FluentBitBaseName, Namespace: SystemNamespaceName}
	FluentBitClusterRole            = types.NamespacedName{Name: FluentBitBaseName, Namespace: SystemNamespaceName}
	FluentBitClusterRoleBinding     = types.NamespacedName{Name: FluentBitBaseName, Namespace: SystemNamespaceName}
	FluentBitExporterMetricsService = types.NamespacedName{Name: FluentBitBaseName + "-exporter-metrics", Namespace: SystemNamespaceName}
	FluentBitMetricsService         = types.NamespacedName{Name: FluentBitBaseName + "-metrics", Namespace: SystemNamespaceName}
	FluentBitConfigMap              = types.NamespacedName{Name: FluentBitBaseName, Namespace: SystemNamespaceName}
	FluentBitSectionsConfigMap      = types.NamespacedName{Name: FluentBitBaseName + "-sections", Namespace: SystemNamespaceName}
	FluentBitLuaConfigMap           = types.NamespacedName{Name: FluentBitBaseName + "-luascripts", Namespace: SystemNamespaceName}
	FluentBitParserConfigMap        = types.NamespacedName{Name: FluentBitBaseName + "-parsers", Namespace: SystemNamespaceName}
	FluentBitFilesConfigMap         = types.NamespacedName{Name: FluentBitBaseName + "-files", Namespace: SystemNamespaceName}
	FluentBitNetworkPolicy          = types.NamespacedName{Name: FluentBitBaseName, Namespace: SystemNamespaceName}

	SelfMonitorName          = types.NamespacedName{Name: SelfMonitorBaseName, Namespace: SystemNamespaceName}
	SelfMonitorNetworkPolicy = types.NamespacedName{Name: SelfMonitorBaseName, Namespace: SystemNamespaceName}

	TelemetryName = types.NamespacedName{Name: DefaultTelemetryName, Namespace: SystemNamespaceName}

	WebhookCertSecret = types.NamespacedName{Name: "telemetry-webhook-cert", Namespace: SystemNamespaceName}
)
