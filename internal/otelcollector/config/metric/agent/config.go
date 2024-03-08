package agent

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/prometheus"
)

type Config struct {
	config.Base `yaml:",inline"`

	Receivers  Receivers  `yaml:"receivers"`
	Processors Processors `yaml:"processors"`
	Exporters  Exporters  `yaml:"exporters"`
}

type Receivers struct {
	KubeletStats          *KubeletStatsReceiver `yaml:"kubeletstats,omitempty"`
	PrometheusAppPods     *PrometheusReceiver   `yaml:"prometheus/app-pods,omitempty"`
	PrometheusAppServices *PrometheusReceiver   `yaml:"prometheus/app-services,omitempty"`
	PrometheusIstio       *PrometheusReceiver   `yaml:"prometheus/istio,omitempty"`
}

type KubeletStatsReceiver struct {
	CollectionInterval string               `yaml:"collection_interval"`
	AuthType           string               `yaml:"auth_type"`
	Endpoint           string               `yaml:"endpoint"`
	InsecureSkipVerify bool                 `yaml:"insecure_skip_verify"`
	MetricGroups       []MetricGroupType    `yaml:"metric_groups"`
	Metrics            KubeletMetricsConfig `yaml:"metrics"`
}

type KubeletMetricConfig struct {
	Enabled bool `yaml:"enabled"`
}

type KubeletMetricsConfig struct {
	ContainerCPUUsage       KubeletMetricConfig `yaml:"container.cpu.usage"`
	ContainerCPUUtilization KubeletMetricConfig `yaml:"container.cpu.utilization"`
	K8sNodeCPUUsage         KubeletMetricConfig `yaml:"k8s.node.cpu.usage"`
	K8sNodeCPUUtilization   KubeletMetricConfig `yaml:"k8s.node.cpu.utilization"`
	K8sPodCPUUsage          KubeletMetricConfig `yaml:"k8s.pod.cpu.usage"`
	K8sPodCPUUtilization    KubeletMetricConfig `yaml:"k8s.pod.cpu.utilization"`
}

type MetricGroupType string

const (
	MetricGroupTypeContainer MetricGroupType = "container"
	MetricGroupTypePod       MetricGroupType = "pod"
)

type PrometheusReceiver struct {
	Config PrometheusConfig `yaml:"config"`
}

type PrometheusConfig struct {
	ScrapeConfigs []prometheus.ScrapeConfig `yaml:"scrape_configs,omitempty"`
}

type Processors struct {
	config.BaseProcessors `yaml:",inline"`

	DeleteServiceName           *config.ResourceProcessor `yaml:"resource/delete-service-name,omitempty"`
	InsertInputSourceRuntime    *config.ResourceProcessor `yaml:"resource/insert-input-source-runtime,omitempty"`
	InsertInputSourcePrometheus *config.ResourceProcessor `yaml:"resource/insert-input-source-prometheus,omitempty"`
	InsertInputSourceIstio      *config.ResourceProcessor `yaml:"resource/insert-input-source-istio,omitempty"`
	DropInternalCommunication   *FilterProcessor          `yaml:"filter/drop-internal-communication,omitempty"`
}

type Exporters struct {
	OTLP config.OTLPExporter `yaml:"otlp"`
}

type FilterProcessor struct {
	Metrics FilterProcessorMetrics `yaml:"metrics"`
}

type FilterProcessorMetrics struct {
	DataPoint []string `yaml:"datapoint,omitempty"`
	Metric    []string `yaml:"metric,omitempty"`
}
