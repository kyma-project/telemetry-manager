package gateway

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
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

	K8sAttributes           *config.K8sAttributesProcessor     `yaml:"k8sattributes,omitempty"`
	InsertClusterAttributes *config.ResourceProcessor          `yaml:"resource/insert-cluster-attributes,omitempty"`
	IstioNoiseFilter        *IstioNoiseFilter                  `yaml:"istio_noise_filter,omitempty"`
	ResolveServiceName      *config.ServiceEnrichmentProcessor `yaml:"service_enrichment,omitempty"`
	DropKymaAttributes      *config.ResourceProcessor          `yaml:"resource/drop-kyma-attributes,omitempty"`
}

type IstioNoiseFilter struct {
}

type Traces struct {
	Span []string `yaml:"span"`
}

type TransformProcessor struct {
	ErrorMode       string                                `yaml:"error_mode"`
	TraceStatements []config.TransformProcessorStatements `yaml:"trace_statements"`
}

type Exporters map[string]Exporter

type Exporter struct {
	OTLP *config.OTLPExporter `yaml:",inline,omitempty"`
}
