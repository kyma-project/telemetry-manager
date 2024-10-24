package runtime

var (
	JobsMetricsNames          = k8sClusterJobMetricsNames
	k8sClusterJobMetricsNames = []string{
		"k8s.job.active_pods",
		"k8s.job.desired_successful_pods",
		"k8s.job.failed_pods",
		"k8s.job.max_parallel_pods",
		"k8s.job.successful_pods",
	}

	JobResourceAttributes = []string{
		"k8s.job.name",
	}
)
