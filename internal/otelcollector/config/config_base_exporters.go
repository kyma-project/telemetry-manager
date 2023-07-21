package config

type OTLPExporter struct {
	Endpoint       string            `yaml:"endpoint,omitempty"`
	Headers        map[string]string `yaml:"headers,omitempty"`
	TLS            TLS               `yaml:"tls,omitempty"`
	SendingQueue   SendingQueue      `yaml:"sending_queue,omitempty"`
	RetryOnFailure RetryOnFailure    `yaml:"retry_on_failure,omitempty"`
}

type TLS struct {
	Insecure bool `yaml:"insecure"`
}

type SendingQueue struct {
	Enabled   bool `yaml:"enabled"`
	QueueSize int  `yaml:"queue_size"`
}

type RetryOnFailure struct {
	Enabled         bool   `yaml:"enabled"`
	InitialInterval string `yaml:"initial_interval"`
	MaxInterval     string `yaml:"max_interval"`
	MaxElapsedTime  string `yaml:"max_elapsed_time"`
}

type LoggingExporter struct {
	Verbosity string `yaml:"verbosity"`
}

func DefaultLoggingExporter() *LoggingExporter {
	return &LoggingExporter{Verbosity: "basic"}
}
