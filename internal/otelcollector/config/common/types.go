package common

// =============================================================================
// BASE CONFIGURATION TYPES
// =============================================================================

// Base represents the root configuration structure for OpenTelemetry Collector
type Base struct {
	Extensions Extensions `yaml:"extensions"`
	Service    Service    `yaml:"service"`
}

// Extensions defines the extensions configuration
type Extensions struct {
	HealthCheck      Endpoint         `yaml:"health_check,omitempty"`
	Pprof            Endpoint         `yaml:"pprof,omitempty"`
	K8sLeaderElector K8sLeaderElector `yaml:"k8s_leader_elector,omitempty"`
}

// Service defines the service configuration including pipelines and telemetry
type Service struct {
	Pipelines  Pipelines `yaml:"pipelines,omitempty"`
	Telemetry  Telemetry `yaml:"telemetry,omitempty"`
	Extensions []string  `yaml:"extensions,omitempty"`
}

// Telemetry defines the telemetry configuration for the collector itself
type Telemetry struct {
	Metrics Metrics `yaml:"metrics"`
	Logs    Logs    `yaml:"logs"`
}

// Metrics defines metrics configuration for telemetry
type Metrics struct {
	Readers []MetricReader `yaml:"readers"`
}

// MetricReader defines a metric reader configuration
type MetricReader struct {
	Pull PullMetricReader `yaml:"pull"`
}

// PullMetricReader defines a pull-based metric reader
type PullMetricReader struct {
	Exporter MetricExporter `yaml:"exporter"`
}

// MetricExporter defines metric exporter configuration
type MetricExporter struct {
	Prometheus PrometheusMetricExporter `yaml:"prometheus"`
}

// PrometheusMetricExporter defines Prometheus exporter configuration
type PrometheusMetricExporter struct {
	Host string `yaml:"host"`
	Port int32  `yaml:"port"`
}

// Logs defines logs configuration for telemetry
type Logs struct {
	Level    string `yaml:"level"`
	Encoding string `yaml:"encoding"`
}

// =============================================================================
// PIPELINE TYPES
// =============================================================================

// Pipelines is a map of pipeline names to pipeline configurations
type Pipelines map[string]Pipeline

// Pipeline defines a processing pipeline with receivers, processors, and exporters
type Pipeline struct {
	Receivers  []string `yaml:"receivers"`
	Processors []string `yaml:"processors"`
	Exporters  []string `yaml:"exporters"`
}

// =============================================================================
// RECEIVER TYPES
// =============================================================================

// OTLPReceiver defines OTLP receiver configuration
type OTLPReceiver struct {
	Protocols ReceiverProtocols `yaml:"protocols,omitempty"`
}

// ReceiverProtocols defines the protocols supported by receivers
type ReceiverProtocols struct {
	HTTP Endpoint `yaml:"http,omitempty"`
	GRPC Endpoint `yaml:"grpc,omitempty"`
}

// =============================================================================
// EXPORTER TYPES
// =============================================================================

// OTLPExporter defines OTLP exporter configuration
type OTLPExporter struct {
	MetricsEndpoint string            `yaml:"metrics_endpoint,omitempty"`
	TracesEndpoint  string            `yaml:"traces_endpoint,omitempty"`
	Endpoint        string            `yaml:"endpoint,omitempty"`
	Headers         map[string]string `yaml:"headers,omitempty"`
	TLS             TLS               `yaml:"tls,omitempty"`
	SendingQueue    SendingQueue      `yaml:"sending_queue,omitempty"`
	RetryOnFailure  RetryOnFailure    `yaml:"retry_on_failure,omitempty"`
}

// TLS defines TLS configuration for exporters
type TLS struct {
	Insecure           bool   `yaml:"insecure"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify,omitempty"`
	CertPem            string `yaml:"cert_pem,omitempty"`
	KeyPem             string `yaml:"key_pem,omitempty"`
	CAPem              string `yaml:"ca_pem,omitempty"`
}

// SendingQueue defines queue configuration for exporters
type SendingQueue struct {
	Enabled   bool `yaml:"enabled"`
	QueueSize int  `yaml:"queue_size"`
}

// RetryOnFailure defines retry configuration for exporters
type RetryOnFailure struct {
	Enabled         bool   `yaml:"enabled"`
	InitialInterval string `yaml:"initial_interval"`
	MaxInterval     string `yaml:"max_interval"`
	MaxElapsedTime  string `yaml:"max_elapsed_time"`
}

// =============================================================================
// PROCESSOR TYPES
// =============================================================================

// BaseProcessors defines basic processor configurations
type BaseProcessors struct {
	Batch         *BatchProcessor `yaml:"batch,omitempty"`
	MemoryLimiter *MemoryLimiter  `yaml:"memory_limiter,omitempty"`
}

// BatchProcessor defines batch processor configuration
type BatchProcessor struct {
	SendBatchSize    int    `yaml:"send_batch_size"`
	Timeout          string `yaml:"timeout"`
	SendBatchMaxSize int    `yaml:"send_batch_max_size"`
}

// MemoryLimiter defines memory limiter processor configuration
type MemoryLimiter struct {
	CheckInterval        string `yaml:"check_interval"`
	LimitPercentage      int    `yaml:"limit_percentage"`
	SpikeLimitPercentage int    `yaml:"spike_limit_percentage"`
}

// K8sAttributesProcessor defines Kubernetes attributes processor configuration
type K8sAttributesProcessor struct {
	AuthType       string             `yaml:"auth_type"`
	Passthrough    bool               `yaml:"passthrough"`
	Extract        ExtractK8sMetadata `yaml:"extract"`
	PodAssociation []PodAssociations  `yaml:"pod_association"`
}

// ExtractK8sMetadata defines metadata extraction configuration
type ExtractK8sMetadata struct {
	Metadata []string       `yaml:"metadata"`
	Labels   []ExtractLabel `yaml:"labels"`
}

// ExtractLabel defines label extraction configuration
type ExtractLabel struct {
	From     string `yaml:"from"`
	Key      string `yaml:"key,omitempty"`
	TagName  string `yaml:"tag_name"`
	KeyRegex string `yaml:"key_regex,omitempty"`
}

// PodAssociations defines pod association configuration
type PodAssociations struct {
	Sources []PodAssociation `yaml:"sources"`
}

// PodAssociation defines individual pod association
type PodAssociation struct {
	From string `yaml:"from"`
	Name string `yaml:"name,omitempty"`
}

// ResourceProcessor defines resource processor configuration
type ResourceProcessor struct {
	Attributes []AttributeAction `yaml:"attributes"`
}

// AttributeAction defines resource attribute modification actions
type AttributeAction struct {
	Action       string `yaml:"action,omitempty"`
	Key          string `yaml:"key,omitempty"`
	Value        string `yaml:"value,omitempty"`
	RegexPattern string `yaml:"pattern,omitempty"`
}

// TransformProcessor defines transform processor configuration
type TransformProcessor struct {
	ErrorMode        string                         `yaml:"error_mode"`
	LogStatements    []TransformProcessorStatements `yaml:"log_statements,omitempty"`
	MetricStatements []TransformProcessorStatements `yaml:"metric_statements,omitempty"`
	TraceStatements  []TransformProcessorStatements `yaml:"trace_statements,omitempty"`
}

// TransformProcessorStatements defines transform processor statements
type TransformProcessorStatements struct {
	Statements []string `yaml:"statements"`
	Conditions []string `yaml:"conditions,omitempty"`
}

// ServiceEnrichmentProcessor defines service enrichment processor configuration
type ServiceEnrichmentProcessor struct {
	ResourceAttributes []string `yaml:"resource_attributes"`
}

// IstioNoiseFilterProcessor defines Istio noise filter processor configuration
type IstioNoiseFilterProcessor struct {
}

// =============================================================================
// UTILITY TYPES
// =============================================================================

// Endpoint defines an endpoint configuration
type Endpoint struct {
	Endpoint string `yaml:"endpoint,omitempty"`
}

// K8sLeaderElector defines Kubernetes leader elector configuration
type K8sLeaderElector struct {
	AuthType       string `yaml:"auth_type"`
	LeaseName      string `yaml:"lease_name"`
	LeaseNamespace string `yaml:"lease_namespace"`
}
