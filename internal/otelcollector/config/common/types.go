package common

// =============================================================================
// BASE CONFIGURATION TYPES
// =============================================================================

// Base represents the root configuration structure for OpenTelemetry Collector
type Base struct {
	Extensions Extensions `yaml:"extensions"`
	Service    Service    `yaml:"service"`
}

type Extensions struct {
	HealthCheck      Endpoint         `yaml:"health_check,omitempty"`
	Pprof            Endpoint         `yaml:"pprof,omitempty"`
	K8sLeaderElector K8sLeaderElector `yaml:"k8s_leader_elector,omitempty"`
}

type Service struct {
	Pipelines  Pipelines `yaml:"pipelines,omitempty"`
	Telemetry  Telemetry `yaml:"telemetry,omitempty"`
	Extensions []string  `yaml:"extensions,omitempty"`
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

// Logs defines logs configuration for telemetry
type Logs struct {
	Level    string `yaml:"level"`
	Encoding string `yaml:"encoding"`
}

// =============================================================================
// PIPELINE TYPES
// =============================================================================

type Pipelines map[string]Pipeline

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
	Endpoint        string            `yaml:"endpoint,omitempty"`
	Headers         map[string]string `yaml:"headers,omitempty"`
	TLS             TLS               `yaml:"tls,omitempty"`
	SendingQueue    SendingQueue      `yaml:"sending_queue,omitempty"`
	RetryOnFailure  RetryOnFailure    `yaml:"retry_on_failure,omitempty"`
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
	Metadata []string       `yaml:"metadata"`
	Labels   []ExtractLabel `yaml:"labels"`
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

type ResourceProcessor struct {
	Attributes []AttributeAction `yaml:"attributes"`
}

type AttributeAction struct {
	Action       string `yaml:"action,omitempty"`
	Key          string `yaml:"key,omitempty"`
	Value        string `yaml:"value,omitempty"`
	RegexPattern string `yaml:"pattern,omitempty"`
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

// =============================================================================
// EXTENSION TYPES
// =============================================================================

type K8sLeaderElector struct {
	AuthType       string `yaml:"auth_type"`
	LeaseName      string `yaml:"lease_name"`
	LeaseNamespace string `yaml:"lease_namespace"`
}
