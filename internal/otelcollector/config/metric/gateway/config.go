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

	CumulativeToDelta           *CumulativeToDeltaConfig `yaml:"cumulativetodelta,omitempty"`
	DropIfInputSourceRuntime    *FilterProcessor         `yaml:"filter/drop-if-input-source-runtime,omitempty"`
	DropIfInputSourcePrometheus *FilterProcessor         `yaml:"filter/drop-if-input-source-prometheus,omitempty"`
}

type FilterProcessor struct {
	Metrics FilterProcessorMetric `yaml:"metrics"`
}

type FilterProcessorMetric struct {
	DataPoint []string `yaml:"datapoint"`
}

type Exporters map[string]Exporter

type Exporter struct {
	OTLP    *config.OTLPExporter    `yaml:",inline,omitempty"`
	Logging *config.LoggingExporter `yaml:",inline,omitempty"`
}

type CumulativeToDeltaConfig struct{}
