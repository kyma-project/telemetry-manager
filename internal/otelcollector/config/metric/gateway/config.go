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

	DropIfInputSourceRuntime    *FilterProcessor            `yaml:"filter/drop-if-input-source-runtime,omitempty"`
	DropIfInputSourcePrometheus *FilterProcessor            `yaml:"filter/drop-if-input-source-prometheus,omitempty"`
	DropIfInputSourceIstio      *FilterProcessor            `yaml:"filter/drop-if-input-source-istio,omitempty"`
	CumulativeToDelta           *CumulativeToDeltaProcessor `yaml:"cumulativetodelta,omitempty"`
	ResolveServiceName          *TransformProcessor         `yaml:"transform/resolve-service-name,omitempty"`
}

type FilterProcessor struct {
	Metrics FilterProcessorMetric `yaml:"metrics"`
}

type FilterProcessorMetric struct {
	DataPoint []string `yaml:"datapoint"`
}

type CumulativeToDeltaProcessor struct{}

type TransformProcessor struct {
	ErrorMode        string                               `yaml:"error_mode"`
	MetricStatements []TransformProcessorMetricStatements `yaml:"metric_statements"`
}

type TransformProcessorMetricStatements struct {
	Context    string   `yaml:"context"`
	Statements []string `yaml:"statements"`
}

type Exporters map[string]Exporter

type Exporter struct {
	OTLP *config.OTLPExporter `yaml:",inline,omitempty"`
}
