package loggateway

type FilterProcessor struct {
	Logs FilterProcessorLogs `yaml:"logs"`
}

type FilterProcessorLogs struct {
	Log []string `yaml:"log_record,omitempty"`
}

type IstioEnrichmentProcessor struct {
	ScopeVersion string `yaml:"scope_version,omitempty"`
}
