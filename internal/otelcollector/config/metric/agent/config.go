package agent

import (
	promconfig "github.com/prometheus/prometheus/config"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

type Config struct {
	config.BaseConfig `yaml:",inline"`

	Receivers  ReceiversConfig  `yaml:"receivers"`
	Processors ProcessorsConfig `yaml:"processors"`
	Exporters  ExportersConfig  `yaml:"exporters"`
}

type ReceiversConfig struct {
	KubeletStats      *KubeletStatsReceiverConfig `yaml:"kubeletstats,omitempty"`
	PrometheusSelf    *PrometheusReceiverConfig   `yaml:"prometheus/self,omitempty"`
	PrometheusAppPods *PrometheusReceiverConfig   `yaml:"prometheus/app-pods,omitempty"`
}

type KubeletStatsReceiverConfig struct {
	CollectionInterval string            `yaml:"collection_interval"`
	AuthType           string            `yaml:"auth_type"`
	Endpoint           string            `yaml:"endpoint"`
	InsecureSkipVerify bool              `yaml:"insecure_skip_verify"`
	MetricGroups       []MetricGroupType `yaml:"metric_groups"`
}

type MetricGroupType string

const (
	MetricGroupTypeContainer MetricGroupType = "container"
	MetricGroupTypePod       MetricGroupType = "pod"
)

type PrometheusReceiverConfig struct {
	Config promconfig.Config `yaml:"config"`
}

type ProcessorsConfig struct {
	DeleteServiceName          *config.ResourceProcessorConfig `yaml:"resource/delete-service-name,omitempty"`
	InsertInputSourceRuntime   *config.ResourceProcessorConfig `yaml:"resource/insert-input-source-runtime,omitempty"`
	InsertInputSourceWorkloads *config.ResourceProcessorConfig `yaml:"resource/insert-input-source-workloads,omitempty"`
}

type ExportersConfig struct {
	OTLP config.OTLPExporterConfig `yaml:"otlp"`
}
