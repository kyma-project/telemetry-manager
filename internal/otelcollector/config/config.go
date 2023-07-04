package config

import (
	promconfig "github.com/prometheus/prometheus/config"
)

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

type ExportersConfig map[string]ExporterConfig

type ExporterConfig struct {
	*OTLPExporterConfig    `yaml:",inline,omitempty"`
	*LoggingExporterConfig `yaml:",inline,omitempty"`
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

type ReceiversConfig struct {
	OpenCensus     *EndpointConfig             `yaml:"opencensus,omitempty"`
	OTLP           *OTLPReceiverConfig         `yaml:"otlp,omitempty"`
	KubeletStats   *KubeletStatsReceiverConfig `yaml:"kubeletstats,omitempty"`
	PrometheusSelf *PrometheusReceiverConfig   `yaml:"prometheus/self,omitempty"`
}

type EndpointConfig struct {
	Endpoint string `yaml:"endpoint,omitempty"`
}

type OTLPReceiverConfig struct {
	Protocols ReceiverProtocols `yaml:"protocols,omitempty"`
}

type ReceiverProtocols struct {
	HTTP EndpointConfig `yaml:"http,omitempty"`
	GRPC EndpointConfig `yaml:"grpc,omitempty"`
}

type KubeletStatsReceiverConfig struct {
	CollectionInterval string            `yaml:"collection_interval,omitempty"`
	AuthType           string            `yaml:"auth_type,omitempty"`
	Endpoint           string            `yaml:"endpoint,omitempty"`
	InsecureSkipVerify bool              `yaml:"insecure_skip_verify,omitempty"`
	MetricGroups       []MetricGroupType `yaml:"metric_groups,omitempty"`
}

type MetricGroupType string

const (
	MetricGroupTypeContainer MetricGroupType = "container"
	MetricGroupTypePod       MetricGroupType = "pod"
)

type PrometheusReceiverConfig struct {
	Config promconfig.Config `yaml:"config,omitempty"`
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

type PipelinesConfig map[string]PipelineConfig

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

type ServiceConfig struct {
	Pipelines  PipelinesConfig `yaml:"pipelines,omitempty"`
	Telemetry  TelemetryConfig `yaml:"telemetry,omitempty"`
	Extensions []string        `yaml:"extensions,omitempty"`
}

type ExtensionsConfig struct {
	HealthCheck EndpointConfig `yaml:"health_check,omitempty"`
	Pprof       EndpointConfig `yaml:"pprof,omitempty"`
}

type Config struct {
	Receivers  ReceiversConfig  `yaml:"receivers"`
	Exporters  ExportersConfig  `yaml:"exporters"`
	Processors ProcessorsConfig `yaml:"processors"`
	Extensions ExtensionsConfig `yaml:"extensions"`
	Service    ServiceConfig    `yaml:"service"`
}
