package agent

import (
	promconfig "github.com/prometheus/prometheus/config"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
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
	Config promconfig.Config `yaml:"config"`
}

type Processors struct {
	DeleteServiceName           *config.ResourceProcessor `yaml:"resource/delete-service-name,omitempty"`
	InsertInputSourceRuntime    *config.ResourceProcessor `yaml:"resource/insert-input-source-runtime,omitempty"`
	InsertInputSourcePrometheus *config.ResourceProcessor `yaml:"resource/insert-input-source-prometheus,omitempty"`
	InsertInputSourceIstio      *config.ResourceProcessor `yaml:"resource/insert-input-source-istio,omitempty"`
}

type Exporters struct {
	OTLP config.OTLPExporter `yaml:"otlp"`
}
