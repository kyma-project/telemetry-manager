package config

type TLSConfig struct {
	Insecure bool `yaml:"insecure"`
}

type SendingQueueConfig struct {
	Enabled   bool `yaml:"enabled"`
	QueueSize int  `yaml:"queue_size"`
}

type RetryOnFailureConfig struct {
	Enabled         bool   `yaml:"enabled"`
	InitialInterval string `yaml:"initial_interval"`
	MaxInterval     string `yaml:"max_interval"`
	MaxElapsedTime  string `yaml:"max_elapsed_time"`
}

type OTLPExporterConfig struct {
	Endpoint       string               `yaml:"endpoint,omitempty"`
	Headers        map[string]string    `yaml:"headers,omitempty"`
	TLS            TLSConfig            `yaml:"tls,omitempty"`
	SendingQueue   SendingQueueConfig   `yaml:"sending_queue,omitempty"`
	RetryOnFailure RetryOnFailureConfig `yaml:"retry_on_failure,omitempty"`
}

type LoggingExporterConfig struct {
	Verbosity string `yaml:"verbosity"`
}

type EndpointConfig struct {
	Endpoint string `yaml:"endpoint,omitempty"`
}

type ReceiverProtocols struct {
	HTTP EndpointConfig `yaml:"http,omitempty"`
	GRPC EndpointConfig `yaml:"grpc,omitempty"`
}

type OTLPReceiverConfig struct {
	Protocols ReceiverProtocols `yaml:"protocols,omitempty"`
}

type ReceiverConfig struct {
	OpenCensus *EndpointConfig     `yaml:"opencensus,omitempty"`
	OTLP       *OTLPReceiverConfig `yaml:"otlp,omitempty"`
}

type BatchProcessorConfig struct {
	SendBatchSize    int    `yaml:"send_batch_size"`
	Timeout          string `yaml:"timeout"`
	SendBatchMaxSize int    `yaml:"send_batch_max_size"`
}

type MemoryLimiterConfig struct {
	CheckInterval        string `yaml:"check_interval"`
	LimitPercentage      int    `yaml:"limit_percentage"`
	SpikeLimitPercentage int    `yaml:"spike_limit_percentage"`
}

type ExtractK8sMetadataConfig struct {
	Metadata []string `yaml:"metadata"`
}

type PodAssociation struct {
	From string `yaml:"from"`
	Name string `yaml:"name,omitempty"`
}

type PodAssociations struct {
	Sources []PodAssociation `yaml:"sources"`
}

type K8sAttributesProcessorConfig struct {
	AuthType       string                   `yaml:"auth_type"`
	Passthrough    bool                     `yaml:"passthrough"`
	Extract        ExtractK8sMetadataConfig `yaml:"extract"`
	PodAssociation []PodAssociations        `yaml:"pod_association"`
}

type AttributeAction struct {
	Action string `yaml:"action"`
	Key    string `yaml:"key"`
	Value  string `yaml:"value"`
}

type ResourceProcessorConfig struct {
	Attributes []AttributeAction `yaml:"attributes"`
}

type ProcessorsConfig struct {
	Batch         *BatchProcessorConfig         `yaml:"batch,omitempty"`
	MemoryLimiter *MemoryLimiterConfig          `yaml:"memory_limiter,omitempty"`
	K8sAttributes *K8sAttributesProcessorConfig `yaml:"k8sattributes,omitempty"`
	Resource      *ResourceProcessorConfig      `yaml:"resource,omitempty"`
	Filter        *FilterProcessorConfig        `yaml:"filter,omitempty"`
}

type FilterProcessorConfig struct {
	Traces TraceConfig `yaml:"traces,omitempty"`
}

type TraceConfig struct {
	Span []string `yaml:"span"`
}

type PipelineConfig struct {
	Receivers  []string `yaml:"receivers"`
	Processors []string `yaml:"processors"`
	Exporters  []string `yaml:"exporters"`
}

type MetricsConfig struct {
	Address string `yaml:"address"`
}

type LoggingConfig struct {
	Level string `yaml:"level"`
}

type TelemetryConfig struct {
	Metrics MetricsConfig `yaml:"metrics"`
	Logs    LoggingConfig `yaml:"logs"`
}

type OTLPServiceConfig struct {
	Pipelines  map[string]PipelineConfig `yaml:"pipelines,omitempty"`
	Telemetry  TelemetryConfig           `yaml:"telemetry,omitempty"`
	Extensions []string                  `yaml:"extensions,omitempty"`
}

type ExtensionsConfig struct {
	HealthCheck EndpointConfig `yaml:"health_check"`
}

type Config struct {
	Receivers  ReceiverConfig    `yaml:"receivers"`
	Exporters  map[string]any    `yaml:"exporters"`
	Processors ProcessorsConfig  `yaml:"processors"`
	Extensions ExtensionsConfig  `yaml:"extensions"`
	Service    OTLPServiceConfig `yaml:"service"`
}
