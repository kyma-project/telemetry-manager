package agent

import (
	"time"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
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

type MetricConfig struct {
	Enabled bool `yaml:"enabled"`
}

type KubeletMetricsConfig struct {
	ContainerCPUUsage       MetricConfig `yaml:"container.cpu.usage"`
	ContainerCPUUtilization MetricConfig `yaml:"container.cpu.utilization"`
	K8sNodeCPUUsage         MetricConfig `yaml:"k8s.node.cpu.usage"`
	K8sNodeCPUUtilization   MetricConfig `yaml:"k8s.node.cpu.utilization"`
	K8sPodCPUUsage          MetricConfig `yaml:"k8s.pod.cpu.usage"`
	K8sPodCPUUtilization    MetricConfig `yaml:"k8s.pod.cpu.utilization"`
}

type MetricGroupType string

const (
	MetricGroupTypeContainer MetricGroupType = "container"
	MetricGroupTypePod       MetricGroupType = "pod"
	MetricGroupTypeNode      MetricGroupType = "node"
)

type PrometheusReceiver struct {
	Config PrometheusConfig `yaml:"config"`
}

type PrometheusConfig struct {
	ScrapeConfigs []ScrapeConfig `yaml:"scrape_configs,omitempty"`
}

type ScrapeConfig struct {
	JobName              string          `yaml:"job_name"`
	SampleLimit          int             `yaml:"sample_limit,omitempty"`
	ScrapeInterval       time.Duration   `yaml:"scrape_interval,omitempty"`
	MetricsPath          string          `yaml:"metrics_path,omitempty"`
	RelabelConfigs       []RelabelConfig `yaml:"relabel_configs,omitempty"`
	MetricRelabelConfigs []RelabelConfig `yaml:"metric_relabel_configs,omitempty"`

	StaticDiscoveryConfigs     []StaticDiscoveryConfig     `yaml:"static_configs,omitempty"`
	KubernetesDiscoveryConfigs []KubernetesDiscoveryConfig `yaml:"kubernetes_sd_configs,omitempty"`

	TLSConfig *TLSConfig `yaml:"tls_config,omitempty"`
}

type TLSConfig struct {
	CAFile             string `yaml:"ca_file,omitempty"`
	CertFile           string `yaml:"cert_file,omitempty"`
	KeyFile            string `yaml:"key_file,omitempty"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

type StaticDiscoveryConfig struct {
	Targets []string `yaml:"targets"`
}

type KubernetesDiscoveryConfig struct {
	Role Role `yaml:"role"`
}

type Role string

const (
	RoleEndpoints Role = "endpoints"
	RolePod       Role = "pod"
)

type RelabelConfig struct {
	SourceLabels []string      `yaml:"source_labels,flow,omitempty"`
	Separator    string        `yaml:"separator,omitempty"`
	Regex        string        `yaml:"regex,omitempty"`
	Modulus      uint64        `yaml:"modulus,omitempty"`
	TargetLabel  string        `yaml:"target_label,omitempty"`
	Replacement  string        `yaml:"replacement,omitempty"`
	Action       RelabelAction `yaml:"action,omitempty"`
}

type RelabelAction string

const (
	Replace RelabelAction = "replace"
	Keep    RelabelAction = "keep"
	Drop    RelabelAction = "drop"
)

type Processors struct {
	config.BaseProcessors `yaml:",inline"`

	DeleteServiceName                 *config.ResourceProcessor  `yaml:"resource/delete-service-name,omitempty"`
	DropInternalCommunication         *FilterProcessor           `yaml:"filter/drop-internal-communication,omitempty"`
	SetInstrumentationScopeRuntime    *metric.TransformProcessor `yaml:"transform/set-instrumentation-scope-runtime,omitempty"`
	SetInstrumentationScopePrometheus *metric.TransformProcessor `yaml:"transform/set-instrumentation-scope-prometheus,omitempty"`
	SetInstrumentationScopeIstio      *metric.TransformProcessor `yaml:"transform/set-instrumentation-scope-istio,omitempty"`
	InsertSkipEnrichmentAttribute     *metric.TransformProcessor `yaml:"transform/insert-skip-enrichment-attribute,omitempty"`
}

type Exporters struct {
	OTLP config.OTLPExporter `yaml:"otlp"`
}

type FilterProcessor struct {
	Metrics FilterProcessorMetrics `yaml:"metrics"`
}

type FilterProcessorMetrics struct {
	Metric []string `yaml:"metric,omitempty"`
}
