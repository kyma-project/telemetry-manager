package overrides

type Config struct {
	Global  GlobalConfig  `yaml:"global,omitempty"`
	Tracing TracingConfig `yaml:"tracing,omitempty"`
	Logging LoggingConfig `yaml:"logging,omitempty"`
	Metrics MetricConfig  `yaml:"metrics,omitempty"`
}

type GlobalConfig struct {
	LogLevel string `yaml:"logLevel,omitempty"`
}

type TracingConfig struct {
	Paused bool `yaml:"paused,omitempty"`
}

type LoggingConfig struct {
	Paused bool `yaml:"paused,omitempty"`
}

type MetricConfig struct {
	Paused bool `yaml:"paused,omitempty"`
}
