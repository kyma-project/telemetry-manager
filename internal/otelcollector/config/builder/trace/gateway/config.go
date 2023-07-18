package gateway

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/common"
)

type Config struct {
	common.BaseConfig

	Receivers  ReceiversConfig         `yaml:"receivers"`
	Processors common.ProcessorsConfig `yaml:"processors"`
	Exporters  common.ExportersConfig  `yaml:"exporters"`
}

type ReceiversConfig struct {
	OpenCensus *common.EndpointConfig     `yaml:"opencensus,omitempty"`
	OTLP       *common.OTLPReceiverConfig `yaml:"otlp,omitempty"`
}
