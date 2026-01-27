package names

const (
	telemetryPrefix       = "telemetry-"
	metricsSuffix         = "-metrics"
	exporterMetricsSuffix = "-exporter-metrics" // used for Fluent Bit directory-size exporter
)

// Agent names
const (
	LogAgent    = telemetryPrefix + "log-agent"
	MetricAgent = telemetryPrefix + "metric-agent"
)

// Gateway names
const (
	LogGateway    = telemetryPrefix + "log-gateway"
	MetricGateway = telemetryPrefix + "metric-gateway"
	TraceGateway  = telemetryPrefix + "trace-gateway"
)

// Fluent Bit resource names
const (
	FluentBitAgent               = telemetryPrefix + "fluent-bit"
	FluentBitSectionsConfigMap   = telemetryPrefix + "fluent-bit-sections"
	FluentBitFilesConfigMap      = telemetryPrefix + "fluent-bit-files"
	FluentBitLuaScriptsConfigMap = telemetryPrefix + "fluent-bit-luascripts"
	FluentBitParsersConfigMap    = telemetryPrefix + "fluent-bit-parsers"
	FluentBitEnvSecret           = telemetryPrefix + "fluent-bit-env"
	FluentBitTLSConfigSecret     = telemetryPrefix + "fluent-bit-output-tls-config"
)

// Metrics Service names (for Prometheus scraping)
const (
	LogAgentMetricsService          = LogAgent + metricsSuffix
	MetricAgentMetricsService       = MetricAgent + metricsSuffix
	TraceGatewayMetricsService      = TraceGateway + metricsSuffix
	LogGatewayMetricsService        = LogGateway + metricsSuffix
	MetricGatewayMetricsService     = MetricGateway + metricsSuffix
	FluentBitMetricsService         = FluentBitAgent + metricsSuffix
	FluentBitExporterMetricsService = FluentBitAgent + exporterMetricsSuffix
)

// OTLP Service names
const (
	OTLPMetricsService = telemetryPrefix + "otlp-metrics"
	OTLPTracesService  = telemetryPrefix + "otlp-traces"
	OTLPLogsService    = telemetryPrefix + "otlp-logs"
)

// Self-monitoring resource names
const (
	SelfMonitor = telemetryPrefix + "self-monitor"
)

// Pipeline lock and sync names
const (
	LogPipelineLock = telemetryPrefix + "logpipeline-lock"
	LogPipelineSync = telemetryPrefix + "logpipeline-sync"

	MetricPipelineLock = telemetryPrefix + "metricpipeline-lock"
	MetricPipelineSync = telemetryPrefix + "metricpipeline-sync"

	TracePipelineLock = telemetryPrefix + "tracepipeline-lock"
	TracePipelineSync = telemetryPrefix + "tracepipeline-sync"
)

// Webhook resource names
const (
	WebhookCertSecret = telemetryPrefix + "webhook-cert"

	// The following resources are not deployed by the telemetry-manager, but patched by it

	WebhookService        = telemetryPrefix + "manager-webhook"
	ValidatingWebhookName = telemetryPrefix + "validating-webhook.kyma-project.io"
	MutatingWebhookName   = telemetryPrefix + "mutating-webhook.kyma-project.io"
)

// Manager resource names
const (
	ManagerMetrics = telemetryPrefix + "manager-metrics"
)

// Priority classes
const (
	PriorityClass     = telemetryPrefix + "priority-class"
	PriorityClassHigh = telemetryPrefix + "priority-class-high"
)

// Network policy names
const (
	ManagerNetworkPolicy = telemetryPrefix + "manager"
)

// MetricsServiceName returns the metrics service name for a given component name
func MetricsServiceName(componentName string) string {
	return componentName + metricsSuffix
}
