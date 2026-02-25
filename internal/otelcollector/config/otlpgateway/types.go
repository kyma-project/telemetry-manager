package otlpgateway

// IstioEnrichmentProcessor enriches Istio access logs with module version.
type IstioEnrichmentProcessor struct {
	ScopeVersion string `yaml:"scope_version,omitempty"`
}
