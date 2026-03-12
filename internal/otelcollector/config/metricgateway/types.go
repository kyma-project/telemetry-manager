package metricgateway

type KymaStatsReceiverConfig struct {
	AuthType           string      `yaml:"auth_type"`
	CollectionInterval string      `yaml:"collection_interval"`
	Resources          []ModuleGVR `yaml:"resources"`
	K8sLeaderElector   string      `yaml:"k8s_leader_elector"`
}

type Metric struct {
	Enabled bool `yaml:"enabled"`
}

type ModuleGVR struct {
	Group    string `yaml:"group"`
	Version  string `yaml:"version"`
	Resource string `yaml:"resource"`
}
