package gateway

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

type Config struct {
	config.Base `yaml:",inline"`

	Receivers Receivers `yaml:"receivers"`
	Exporters Exporters `yaml:"exporters"`
}

type Receivers struct {
	OTLP config.OTLPReceiver `yaml:"otlp"`
}

type Exporters map[string]Exporter

type Exporter struct {
	OTLP *config.OTLPExporter `yaml:",inline,omitempty"`
}
