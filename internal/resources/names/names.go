package names

const telemetryPrefix = "telemetry-"

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
	WebhookService        = telemetryPrefix + "manager-webhook"
	WebhookCertSecret     = telemetryPrefix + "webhook-cert"
	ValidatingWebhookName = telemetryPrefix + "validating-webhook.kyma-project.io"
	MutatingWebhookName   = telemetryPrefix + "mutating-webhook.kyma-project.io"
)

// Manager resource names
const (
	Manager        = telemetryPrefix + "manager"
	ManagerMetrics = telemetryPrefix + "manager-metrics"
)

// Configuration ConfigMaps
const (
	MetricPipelinesConfig = telemetryPrefix + "metricpipelines"
	LogPipelinesConfig    = telemetryPrefix + "logpipelines"
	TracePipelinesConfig  = telemetryPrefix + "tracepipelines"
	ModuleConfig          = telemetryPrefix + "module"
	OverrideConfig        = telemetryPrefix + "override-config"
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
