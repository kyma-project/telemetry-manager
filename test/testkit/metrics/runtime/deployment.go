package runtime

var (
	DeploymentMetricsNames             = kubeletStatsDeploymentMetricsNames
	kubeletStatsDeploymentMetricsNames = []string{
		"k8s.deployment.available",
		"k8s.deployment.desired",
	}

	DeploymentResourceAttributes = []string{
		"k8s.deployment.name",
		"k8s.namespace.name",
	}
)
