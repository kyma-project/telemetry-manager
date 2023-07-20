package common

type OTLPExporterConfig struct {
	Endpoint       string               `yaml:"endpoint,omitempty"`
	Headers        map[string]string    `yaml:"headers,omitempty"`
	TLS            TLSConfig            `yaml:"tls,omitempty"`
	SendingQueue   SendingQueueConfig   `yaml:"sending_queue,omitempty"`
	RetryOnFailure RetryOnFailureConfig `yaml:"retry_on_failure,omitempty"`
}

type TLSConfig struct {
	Insecure bool `yaml:"insecure"`
}

type SendingQueueConfig struct {
	Enabled   bool `yaml:"enabled"`
	QueueSize int  `yaml:"queue_size"`
}

type RetryOnFailureConfig struct {
	Enabled         bool   `yaml:"enabled"`
	InitialInterval string `yaml:"initial_interval"`
	MaxInterval     string `yaml:"max_interval"`
	MaxElapsedTime  string `yaml:"max_elapsed_time"`
}

type LoggingExporterConfig struct {
	Verbosity string `yaml:"verbosity"`
}
