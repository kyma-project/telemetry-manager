package otlpgateway

// IstioEnrichmentProcessor enriches Istio access logs with module version.
type IstioEnrichmentProcessor struct {
	ScopeVersion string `yaml:"scope_version,omitempty"`
}

// KymaStatsReceiver configures the kymastats receiver for collecting Kyma-specific metrics.
type KymaStatsReceiver struct {
	AuthType           string      `yaml:"auth_type"`
	CollectionInterval string      `yaml:"collection_interval"`
	Resources          []ModuleGVR `yaml:"resources"`
	K8sLeaderElector   string      `yaml:"k8s_leader_elector"`
}

// ModuleGVR represents a Kubernetes Group/Version/Resource for the kymastats receiver.
type ModuleGVR struct {
	Group    string `yaml:"group"`
	Version  string `yaml:"version"`
	Resource string `yaml:"resource"`
}
