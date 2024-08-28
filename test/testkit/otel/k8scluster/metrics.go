package k8scluster

import "slices"

var DefaultMetricsNames = slices.Concat(ContainerMetricsNames, PodMetricsNames)

var (
	ContainerMetricsNames = []string{
		"k8s.container.cpu_request",
		"k8s.container.cpu_limit",
		"k8s.container.memory_request",
		"k8s.container.memory_limit",
	}

	PodMetricsNames = []string{
		"k8s.pod.phase",
	}
)
