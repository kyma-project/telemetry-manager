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
	OTLP common.OTLPReceiverConfig `yaml:"otlp"`
}

type ProcessorsConfig struct {
	common.BaseProcessorsConfig `yaml:",inline"`

	FilterByInput map[string]FilterProcessorConfig `yaml:",inline"`
}

type FilterProcessorConfig struct {
	Metrics FilterProcessorMetricConfig `yaml:"metrics"`
}

type FilterProcessorMetricConfig struct {
	DataPoint []string `yaml:"datapoint"`
}

type ExportersConfig map[string]ExporterConfig

type ExporterConfig struct {
	common.BaseGatewayExporterConfig `yaml:",inline"`
}
