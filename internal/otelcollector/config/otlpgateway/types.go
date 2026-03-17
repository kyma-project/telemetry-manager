package otlpgateway

// IstioEnrichmentProcessorConfig enriches Istio access logs with module version.
type IstioEnrichmentProcessorConfig struct {
	ScopeVersion string `yaml:"scope_version,omitempty"`
}
