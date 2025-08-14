package tracegateway

import "github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"

type Config struct {
	common.Base `yaml:",inline"`

	Receivers  map[string]any `yaml:"receivers"`
	Processors map[string]any `yaml:"processors"`
	Exporters  map[string]any `yaml:"exporters"`
}
