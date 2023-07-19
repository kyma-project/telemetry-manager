package common

type BaseConfig struct {
	Extensions ExtensionsConfig `yaml:"extensions"`
	Service    ServiceConfig    `yaml:"service"`
}

type ExtensionsConfig struct {
	HealthCheck EndpointConfig `yaml:"health_check,omitempty"`
	Pprof       EndpointConfig `yaml:"pprof,omitempty"`
}

type EndpointConfig struct {
	Endpoint string `yaml:"endpoint,omitempty"`
}

type ServiceConfig struct {
	Pipelines  PipelinesConfig `yaml:"pipelines,omitempty"`
	Telemetry  TelemetryConfig `yaml:"telemetry,omitempty"`
	Extensions []string        `yaml:"extensions,omitempty"`
}

type PipelinesConfig map[string]PipelineConfig

type PipelineConfig struct {
	Receivers  []string `yaml:"receivers"`
	Processors []string `yaml:"processors"`
	Exporters  []string `yaml:"exporters"`
}

type TelemetryConfig struct {
	Metrics MetricsConfig `yaml:"metrics"`
	Logs    LoggingConfig `yaml:"logs"`
}

type MetricsConfig struct {
	Address string `yaml:"address"`
}

type LoggingConfig struct {
	Level string `yaml:"level"`
}
