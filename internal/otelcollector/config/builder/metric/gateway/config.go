package gateway

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/common"
)

type Config struct {
	common.BaseConfig

	Receivers  ReceiversConfig         `yaml:"receivers"`
	Exporters  common.ExportersConfig  `yaml:"exporters"`
	Processors common.ProcessorsConfig `yaml:"processors"`
}

type ReceiversConfig struct {
	OTLP *common.OTLPReceiverConfig `yaml:"otlp,omitempty"`
}
