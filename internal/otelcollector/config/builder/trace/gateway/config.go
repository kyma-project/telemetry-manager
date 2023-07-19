package gateway

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/common"
)

type Config struct {
	common.BaseConfig `yaml:",inline"`

	Receivers  ReceiversConfig  `yaml:"receivers"`
	Processors ProcessorsConfig `yaml:"processors"`
	Exporters  ExportersConfig  `yaml:"exporters"`
}

type ReceiversConfig struct {
	OpenCensus common.EndpointConfig     `yaml:"opencensus"`
	OTLP       common.OTLPReceiverConfig `yaml:"otlp"`
}

type ProcessorsConfig struct {
	common.BaseProcessorsConfig `yaml:",inline"`

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
	common.BaseGatewayExporterConfig `yaml:",inline"`
}
