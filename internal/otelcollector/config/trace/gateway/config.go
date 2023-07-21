package gateway

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

type Config struct {
	config.BaseConfig `yaml:",inline"`

	Receivers  ReceiversConfig  `yaml:"receivers"`
	Processors ProcessorsConfig `yaml:"processors"`
	Exporters  ExportersConfig  `yaml:"exporters"`
}

type ReceiversConfig struct {
	OpenCensus config.EndpointConfig     `yaml:"opencensus"`
	OTLP       config.OTLPReceiverConfig `yaml:"otlp"`
}

type ProcessorsConfig struct {
	config.BaseProcessorsConfig `yaml:",inline"`

	SpanFilter FilterProcessorConfig `yaml:"filter"`
}

type FilterProcessorConfig struct {
	Traces TraceConfig `yaml:"traces"`
}

type TraceConfig struct {
	Span []string `yaml:"span"`
}

type ExportersConfig map[string]ExporterConfig

type ExporterConfig struct {
	OTLP    *config.OTLPExporterConfig    `yaml:",inline,omitempty"`
	Logging *config.LoggingExporterConfig `yaml:",inline,omitempty"`
}
