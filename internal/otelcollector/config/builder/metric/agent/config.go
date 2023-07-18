package agent

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	promconfig "github.com/prometheus/prometheus/config"
)

type Config struct {
	Receivers  ReceiversConfig         `yaml:"receivers"`
	Processors ProcessorsConfig        `yaml:"processors"`
	Exporters  config.ExportersConfig  `yaml:"exporters"`
	Extensions config.ExtensionsConfig `yaml:"extensions"`
	Service    config.ServiceConfig    `yaml:"service"`
}

type ReceiversConfig struct {
	KubeletStats      *KubeletStatsReceiverConfig `yaml:"kubeletstats,omitempty"`
	PrometheusSelf    *PrometheusReceiverConfig   `yaml:"prometheus/self,omitempty"`
	PrometheusAppPods *PrometheusReceiverConfig   `yaml:"prometheus/app-pods,omitempty"`
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

type ProcessorsConfig struct {
	DropServiceName    *config.ResourceProcessorConfig `yaml:"resource/drop-service-name,omitempty"`
	EmittedByRuntime   *config.ResourceProcessorConfig `yaml:"resource/emitted-by-runtime,omitempty"`
	EmittedByWorkloads *config.ResourceProcessorConfig `yaml:"resource/emitted-by-workloads,omitempty"`
}
