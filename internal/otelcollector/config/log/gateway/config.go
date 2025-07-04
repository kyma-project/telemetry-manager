package gateway

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/log"
)

type Config struct {
	config.Base `yaml:",inline"`

	Receivers  Receivers  `yaml:"receivers"`
	Processors Processors `yaml:"processors"`
	Exporters  Exporters  `yaml:"exporters"`
}

type Receivers struct {
	OTLP config.OTLPReceiver `yaml:"otlp"`
}

type Processors struct {
	config.BaseProcessors `yaml:",inline"`

	SetObsTimeIfZero        *log.TransformProcessor            `yaml:"transform/set-observed-time-if-zero,omitempty"`
	K8sAttributes           *config.K8sAttributesProcessor     `yaml:"k8sattributes,omitempty"`
	IstioNoiseFilter        *config.IstioNoiseFilterProcessor  `yaml:"istio_noise_filter,omitempty"`
	InsertClusterAttributes *config.ResourceProcessor          `yaml:"resource/insert-cluster-attributes,omitempty"`
	ResolveServiceName      *config.ServiceEnrichmentProcessor `yaml:"service_enrichment,omitempty"`
	DropKymaAttributes      *config.ResourceProcessor          `yaml:"resource/drop-kyma-attributes,omitempty"`
	DropIfInputSourceOTLP   *FilterProcessor                   `yaml:"filter/drop-if-input-source-otlp,omitempty"`
	IstioEnrichment         *IstioEnrichmentProcessor          `yaml:"istio_enrichment,omitempty"`

	// NamespaceFilters contains filter processors, which need different configurations per pipeline
	NamespaceFilters NamespaceFilters `yaml:",inline,omitempty"`
}

type NamespaceFilters map[string]*FilterProcessor

type FilterProcessor struct {
	Logs FilterProcessorLogs `yaml:"logs"`
}

type FilterProcessorLogs struct {
	Log []string `yaml:"log_record,omitempty"`
}

type Exporters map[string]Exporter

type Exporter struct {
	OTLP *config.OTLPExporter `yaml:",inline,omitempty"`
}

type IstioEnrichmentProcessor struct {
	ScopeVersion string `yaml:"scope_version,omitempty"`
}
