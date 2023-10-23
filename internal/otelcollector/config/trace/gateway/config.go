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
	OpenCensus config.Endpoint     `yaml:"opencensus"`
	OTLP       config.OTLPReceiver `yaml:"otlp"`
}

type Processors struct {
	config.BaseProcessors `yaml:",inline"`

	SpanFilter         FilterProcessor     `yaml:"filter"`
	ResolveServiceName *TransformProcessor `yaml:"transform/resolve-service-name,omitempty"`
}

type FilterProcessor struct {
	Traces Traces `yaml:"traces"`
}

type Traces struct {
	Span []string `yaml:"span"`
}

type TransformProcessor struct {
	ErrorMode        string                              `yaml:"error_mode"`
	MetricStatements []TransformProcessorTraceStatements `yaml:"trace_statements"`
}

type TransformProcessorTraceStatements struct {
	Context    string   `yaml:"context"`
	Statements []string `yaml:"statements"`
}

type Exporters map[string]Exporter

type Exporter struct {
	OTLP *config.OTLPExporter `yaml:",inline,omitempty"`
}
