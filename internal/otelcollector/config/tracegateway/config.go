package tracegateway

import "github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"

type Config struct {
	common.Base `yaml:",inline"`

	Receivers  Receivers  `yaml:"receivers"`
	Processors Processors `yaml:"processors"`
	Exporters  Exporters  `yaml:"exporters"`
}

type Receivers struct {
	OTLP common.OTLPReceiver `yaml:"otlp"`
}

type Processors struct {
	common.BaseProcessors `yaml:",inline"`

	// OTel Collector components with static IDs
	K8sAttributes           *common.K8sAttributesProcessor     `yaml:"k8sattributes,omitempty"`
	InsertClusterAttributes *common.ResourceProcessor          `yaml:"resource/insert-cluster-attributes,omitempty"`
	IstioNoiseFilter        *common.IstioNoiseFilterProcessor  `yaml:"istio_noise_filter,omitempty"`
	ResolveServiceName      *common.ServiceEnrichmentProcessor `yaml:"service_enrichment,omitempty"`
	DropKymaAttributes      *common.ResourceProcessor          `yaml:"resource/drop-kyma-attributes,omitempty"`

	// OTel Collector components with dynamic IDs that are pipeline name based
	Dynamic map[string]any `yaml:",inline,omitempty"`
}

type Exporters map[string]Exporter

type Exporter struct {
	OTLP *common.OTLPExporter `yaml:",inline,omitempty"`
}
