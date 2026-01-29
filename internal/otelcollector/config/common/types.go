package common

// =============================================================================
// BASE CONFIGURATION TYPES
// =============================================================================

// Config represents the root configuration structure for OpenTelemetry Collector
type Config struct {
	Extensions map[string]any `yaml:"extensions"`
	Service    Service        `yaml:"service"`

	Receivers  map[string]any `yaml:"receivers"`
	Processors map[string]any `yaml:"processors"`
	Exporters  map[string]any `yaml:"exporters"`
	Connectors map[string]any `yaml:"connectors,omitempty"` // Connectors are optional and may not be present in all configurations
}

// =============================================================================
// EXTENSION TYPES
// =============================================================================

type K8sLeaderElectorExtension struct {
	AuthType       string `yaml:"auth_type,omitempty"`
	LeaseName      string `yaml:"lease_name,omitempty"`
	LeaseNamespace string `yaml:"lease_namespace,omitempty"`
}

type FileStorageExtension struct {
	CreateDirectory bool   `yaml:"create_directory,omitempty"`
	Directory       string `yaml:"directory,omitempty"`
}

type OAuth2Extension struct {
	TokenURL     string            `yaml:"token_url"`
	ClientID     string            `yaml:"client_id"`
	ClientSecret string            `yaml:"client_secret"`
	Scopes       []string          `yaml:"scopes,omitempty"`
	Params       map[string]string `yaml:"endpoint_params,omitempty"`
}

// =============================================================================
// SERVICE TYPES
// =============================================================================

type Service struct {
	Pipelines  map[string]Pipeline `yaml:"pipelines,omitempty"`
	Telemetry  Telemetry           `yaml:"telemetry,omitempty"`
	Extensions []string            `yaml:"extensions,omitempty"`
}

type Telemetry struct {
	Metrics Metrics `yaml:"metrics"`
	Logs    Logs    `yaml:"logs"`
}

type Metrics struct {
	Readers []MetricReader `yaml:"readers"`
}

type MetricReader struct {
	Pull PullMetricReader `yaml:"pull"`
}

type PullMetricReader struct {
	Exporter MetricExporter `yaml:"exporter"`
}

type MetricExporter struct {
	Prometheus PrometheusMetricExporter `yaml:"prometheus"`
}

type PrometheusMetricExporter struct {
	Host string `yaml:"host"`
	Port int32  `yaml:"port"`
}

type Logs struct {
	Level    string `yaml:"level"`
	Encoding string `yaml:"encoding"`
}

type Pipeline struct {
	Receivers  []string `yaml:"receivers"`
	Processors []string `yaml:"processors"`
	Exporters  []string `yaml:"exporters"`
}

// =============================================================================
// RECEIVER TYPES
// =============================================================================

type OTLPReceiver struct {
	Protocols ReceiverProtocols `yaml:"protocols,omitempty"`
}

type ReceiverProtocols struct {
	HTTP Endpoint `yaml:"http,omitempty"`
	GRPC Endpoint `yaml:"grpc,omitempty"`
}

type Endpoint struct {
	Endpoint string `yaml:"endpoint,omitempty"`
}

// =============================================================================
// EXPORTER TYPES
// =============================================================================

type OTLPExporter struct {
	MetricsEndpoint string            `yaml:"metrics_endpoint,omitempty"`
	TracesEndpoint  string            `yaml:"traces_endpoint,omitempty"`
	LogsEndpoint    string            `yaml:"logs_endpoint,omitempty"`
	Endpoint        string            `yaml:"endpoint,omitempty"`
	Headers         map[string]string `yaml:"headers,omitempty"`
	TLS             TLS               `yaml:"tls,omitempty"`
	SendingQueue    SendingQueue      `yaml:"sending_queue,omitempty"`
	RetryOnFailure  RetryOnFailure    `yaml:"retry_on_failure,omitempty"`
	Auth            Auth              `yaml:"auth,omitempty"`
}

type TLS struct {
	Insecure           bool   `yaml:"insecure"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify,omitempty"`
	CertPem            string `yaml:"cert_pem,omitempty"`
	KeyPem             string `yaml:"key_pem,omitempty"`
	CAPem              string `yaml:"ca_pem,omitempty"`
}

type SendingQueue struct {
	Enabled   bool `yaml:"enabled"`
	QueueSize int  `yaml:"queue_size"`
}

type RetryOnFailure struct {
	Enabled         bool   `yaml:"enabled"`
	InitialInterval string `yaml:"initial_interval"`
	MaxInterval     string `yaml:"max_interval"`
	MaxElapsedTime  string `yaml:"max_elapsed_time"`
}

type Auth struct {
	Authenticator string `yaml:"authenticator"`
}

// =============================================================================
// PROCESSOR TYPES
// =============================================================================

type BaseProcessors struct {
	Batch         *BatchProcessor `yaml:"batch,omitempty"`
	MemoryLimiter *MemoryLimiter  `yaml:"memory_limiter,omitempty"`
}

type BatchProcessor struct {
	SendBatchSize    int    `yaml:"send_batch_size"`
	Timeout          string `yaml:"timeout"`
	SendBatchMaxSize int    `yaml:"send_batch_max_size"`
}

type MemoryLimiter struct {
	CheckInterval        string `yaml:"check_interval"`
	LimitPercentage      int    `yaml:"limit_percentage"`
	SpikeLimitPercentage int    `yaml:"spike_limit_percentage"`
}

type K8sAttributesProcessor struct {
	AuthType       string             `yaml:"auth_type"`
	Passthrough    bool               `yaml:"passthrough"`
	Extract        ExtractK8sMetadata `yaml:"extract"`
	PodAssociation []PodAssociations  `yaml:"pod_association"`
}

type ExtractK8sMetadata struct {
	Metadata                     []string       `yaml:"metadata"`
	Labels                       []ExtractLabel `yaml:"labels"`
	OTelAnnotations              bool           `yaml:"otel_annotations,omitempty"`
	DeploymentNameFromReplicaset bool           `yaml:"deployment_name_from_replicaset,omitempty"`
}

type ExtractLabel struct {
	From     string `yaml:"from"`
	Key      string `yaml:"key,omitempty"`
	TagName  string `yaml:"tag_name"`
	KeyRegex string `yaml:"key_regex,omitempty"`
}

type PodAssociations struct {
	Sources []PodAssociation `yaml:"sources"`
}

type PodAssociation struct {
	From string `yaml:"from"`
	Name string `yaml:"name,omitempty"`
}

type TransformProcessor struct {
	ErrorMode        string                         `yaml:"error_mode"`
	LogStatements    []TransformProcessorStatements `yaml:"log_statements,omitempty"`
	MetricStatements []TransformProcessorStatements `yaml:"metric_statements,omitempty"`
	TraceStatements  []TransformProcessorStatements `yaml:"trace_statements,omitempty"`
}

type TransformProcessorStatements struct {
	Statements []string `yaml:"statements"`
	Conditions []string `yaml:"conditions,omitempty"`
}

type ServiceEnrichmentProcessor struct {
	ResourceAttributes []string `yaml:"resource_attributes"`
}

type IstioNoiseFilterProcessor struct {
}

type FilterProcessor struct {
	ErrorMode string                 `yaml:"error_mode"`
	Metrics   FilterProcessorMetrics `yaml:"metrics,omitempty"`
	Logs      FilterProcessorLogs    `yaml:"logs,omitempty"`
	Traces    FilterProcessorTraces  `yaml:"traces,omitempty"`
}

type FilterProcessorMetrics struct {
	Metric    []string `yaml:"metric,omitempty"`
	Datapoint []string `yaml:"datapoint,omitempty"`
}

type FilterProcessorTraces struct {
	Span      []string `yaml:"span,omitempty"`
	SpanEvent []string `yaml:"spanevent,omitempty"`
}

type FilterProcessorLogs struct {
	Log []string `yaml:"log_record,omitempty"`
}

// =============================================================================
// CONNECTOR TYPES
// =============================================================================

type RoutingConnector struct {
	DefaultPipelines []string                     `yaml:"default_pipelines"`
	ErrorMode        string                       `yaml:"error_mode"`
	Table            []RoutingConnectorTableEntry `yaml:"table"`
}

type RoutingConnectorTableEntry struct {
	Statement string   `yaml:"statement"`
	Pipelines []string `yaml:"pipelines"`
	Context   string   `yaml:"context,omitempty"`
}

type ForwardConnector struct {
}
