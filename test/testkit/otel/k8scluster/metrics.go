package k8scluster

var (
	ContainerMetricsNames = []string{
		"k8s.container.cpu_request",
		"k8s.container.cpu_limit",
		"k8s.container.memory_request",
		"k8s.container.memory_limit",
		"k8s.container.storage_request",
		"k8s.container.storage_limit",
		"k8s.container.ephemeralstorage_request",
		"k8s.container.ephemeralstorage_limit",
		"k8s.container.restarts",
		"k8s.container.ready",
	}

	PodMetricsNames = []string{
		"k8s.pod.phase",
	}
)
