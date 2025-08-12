package metric

type LeaderElection struct {
	LeaseName      string `yaml:"lease_name"`
	LeaseNamespace string `yaml:"lease_namespace"`
}
