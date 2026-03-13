package loggateway

type FilterProcessorConfig struct {
	Logs FilterProcessorLogs `yaml:"logs"`
}

type FilterProcessorLogs struct {
	Log []string `yaml:"log_record,omitempty"`
}

type IstioEnrichmentProcessorConfig struct {
	ScopeVersion string `yaml:"scope_version,omitempty"`
}
