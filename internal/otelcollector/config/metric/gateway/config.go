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

	K8sAttributes               *config.K8sAttributesProcessor `yaml:"k8sattributes,omitempty"`
	InsertClusterName           *config.ResourceProcessor      `yaml:"resource/insert-cluster-name,omitempty"`
	DropIfInputSourceRuntime    *FilterProcessor               `yaml:"filter/drop-if-input-source-runtime,omitempty"`
	DropIfInputSourcePrometheus *FilterProcessor               `yaml:"filter/drop-if-input-source-prometheus,omitempty"`
	DropIfInputSourceIstio      *FilterProcessor               `yaml:"filter/drop-if-input-source-istio,omitempty"`
	ResolveServiceName          *TransformProcessor            `yaml:"transform/resolve-service-name,omitempty"`
	DropKymaAttributes          *config.ResourceProcessor      `yaml:"resource/drop-kyma-attributes,omitempty"`
}

type FilterProcessor struct {
	Metrics FilterProcessorMetric `yaml:"metrics"`
}

type FilterProcessorMetric struct {
	DataPoint []string `yaml:"datapoint"`
}

type TransformProcessor struct {
	ErrorMode        string                                `yaml:"error_mode"`
	MetricStatements []config.TransformProcessorStatements `yaml:"metric_statements"`
}

type Exporters map[string]Exporter

type Exporter struct {
	OTLP *config.OTLPExporter `yaml:",inline,omitempty"`
}
