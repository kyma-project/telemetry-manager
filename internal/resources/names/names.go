package names

// Agent names
const (
	LogAgent    = "telemetry-log-agent"
	MetricAgent = "telemetry-metric-agent"
)

// Gateway names
const (
	LogGateway    = "telemetry-log-gateway"
	MetricGateway = "telemetry-metric-gateway"
	TraceGateway  = "telemetry-trace-gateway"
)

// Fluent Bit resource names
const (
	FluentBitAgent               = "telemetry-fluent-bit"
	FluentBitSectionsConfigMap   = "telemetry-fluent-bit-sections"
	FluentBitFilesConfigMap      = "telemetry-fluent-bit-files"
	FluentBitLuaScriptsConfigMap = "telemetry-fluent-bit-luascripts"
	FluentBitParsersConfigMap    = "telemetry-fluent-bit-parsers"
	FluentBitEnvSecret           = "telemetry-fluent-bit-env"
	FluentBitTLSConfigSecret     = "telemetry-fluent-bit-output-tls-config"
)

// OTLP Service names
const (
	OTLPMetricsService = "telemetry-otlp-metrics"
	OTLPTracesService  = "telemetry-otlp-traces"
	OTLPLogsService    = "telemetry-otlp-logs"
)

// Self-monitoring resource names
const (
	SelfMonitor = "telemetry-self-monitor"
)

// Pipeline lock and sync names
const (
	LogPipelineLock = "telemetry-logpipeline-lock"
	LogPipelineSync = "telemetry-logpipeline-sync"

	MetricPipelineLock = "telemetry-metricpipeline-lock"
	MetricPipelineSync = "telemetry-metricpipeline-sync"

	TracePipelineLock = "telemetry-tracepipeline-lock"
	TracePipelineSync = "telemetry-tracepipeline-sync"
)

// Webhook resource names
const (
	WebhookService        = "telemetry-manager-webhook"
	WebhookCertSecret     = "telemetry-webhook-cert"
	ValidatingWebhookName = "telemetry-validating-webhook.kyma-project.io"
	MutatingWebhookName   = "telemetry-mutating-webhook.kyma-project.io"
)

// Manager resource names
const (
	Manager        = "telemetry-manager"
	ManagerMetrics = "telemetry-manager-metrics"
)

// Configuration ConfigMaps
const (
	MetricPipelinesConfig = "telemetry-metricpipelines"
	LogPipelinesConfig    = "telemetry-logpipelines"
	TracePipelinesConfig  = "telemetry-tracepipelines"
	ModuleConfig          = "telemetry-module"
	OverrideConfig        = "telemetry-override-config"
)

// Priority classes
const (
	PriorityClass     = "telemetry-priority-class"
	PriorityClassHigh = "telemetry-priority-class-high"
)

// Network policy names
const (
	ManagerNetworkPolicy = "telemetry-manager"
)
