package gateway

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

type Config struct {
	Receivers  ReceiversConfig         `yaml:"receivers"`
	Exporters  config.ExportersConfig  `yaml:"exporters"`
	Processors config.ProcessorsConfig `yaml:"processors"`
	Extensions config.ExtensionsConfig `yaml:"extensions"`
	Service    config.ServiceConfig    `yaml:"service"`
}

type ReceiversConfig struct {
	OTLP *config.OTLPReceiverConfig `yaml:"otlp,omitempty"`
}
