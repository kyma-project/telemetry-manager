package agent

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"time"
)

type Config struct {
	config.Base `yaml:",inline"`

	Receivers  Receivers  `yaml:"receivers"`
	Processors Processors `yaml:"processors"`
	Exporters  Exporters  `yaml:"exporters"`
}

type Receivers struct {
	KubeletStats      *KubeletStatsReceiver `yaml:"kubeletstats,omitempty"`
	PrometheusSelf    *PrometheusReceiver   `yaml:"prometheus/self,omitempty"`
	PrometheusAppPods *PrometheusReceiver   `yaml:"prometheus/app-pods,omitempty"`
	PrometheusIstio   *PrometheusReceiver   `yaml:"prometheus/istio,omitempty"`
}

type KubeletStatsReceiver struct {
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

type PrometheusReceiver struct {
	Config PrometheusConfig `yaml:"config"`
}

type PrometheusConfig struct {
	ScrapeConfigs []ScrapeConfig `yaml:"scrape_configs,omitempty"`
}

type ScrapeConfig struct {
	JobName              string          `yaml:"job_name"`
	ScrapeInterval       time.Duration   `yaml:"scrape_interval,omitempty"`
	MetricsPath          string          `yaml:"metrics_path,omitempty"`
	RelabelConfigs       []RelabelConfig `yaml:"relabel_configs,omitempty"`
	MetricRelabelConfigs []RelabelConfig `yaml:"metric_relabel_configs,omitempty"`

	StaticDiscoveryConfigs     []StaticDiscoveryConfig     `yaml:"static_configs,omitempty"`
	KubernetesDiscoveryConfigs []KubernetesDiscoveryConfig `yaml:"kubernetes_sd_configs,omitempty"`
}

type StaticDiscoveryConfig struct {
	Targets []string `yaml:"targets"`
}

type KubernetesDiscoveryConfig struct {
	Role Role `yaml:"role"`
}

type Role string

const (
	RolePod Role = "pod"
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
	DeleteServiceName           *config.ResourceProcessor `yaml:"resource/delete-service-name,omitempty"`
	InsertInputSourceRuntime    *config.ResourceProcessor `yaml:"resource/insert-input-source-runtime,omitempty"`
	InsertInputSourcePrometheus *config.ResourceProcessor `yaml:"resource/insert-input-source-prometheus,omitempty"`
	InsertInputSourceIstio      *config.ResourceProcessor `yaml:"resource/insert-input-source-istio,omitempty"`
}

type Exporters struct {
	OTLP config.OTLPExporter `yaml:"otlp"`
}
