package config

type OTLPExporter struct {
	MetricsEndpoint string            `yaml:"metrics_endpoint,omitempty"`
	TracesEndpoint  string            `yaml:"traces_endpoint,omitempty"`
	Endpoint        string            `yaml:"endpoint,omitempty"`
	Headers         map[string]string `yaml:"headers,omitempty"`
	TLS             TLS               `yaml:"tls,omitempty"`
	SendingQueue    SendingQueue      `yaml:"sending_queue,omitempty"`
	RetryOnFailure  RetryOnFailure    `yaml:"retry_on_failure,omitempty"`
}

type TLS struct {
	Insecure           bool   `yaml:"insecure"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify,omitempty"`
	CertPem            string `yaml:"cert_pem,omitempty"`
	KeyPem             string `yaml:"key_pem,omitempty"`
	CAPem              string `yaml:"ca_pem,omitempty"`
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
