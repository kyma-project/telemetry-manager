package runtime

import "slices"

var (
	ContainerMetricsNames = slices.Concat(kubeletstatsContainerMetricsNames, k8sclusterContainerMeticsNames)

	kubeletstatsContainerMetricsNames = []string{
		"container.cpu.time",
		"container.cpu.usage",
		"container.filesystem.available",
		"container.filesystem.capacity",
		"container.filesystem.usage",
		"container.memory.available",
		"container.memory.major_page_faults",
		"container.memory.page_faults",
		"container.memory.rss",
		"container.memory.usage",
		"container.memory.working_set",
	}

	k8sclusterContainerMeticsNames = []string{
		"k8s.container.cpu_request",
		"k8s.container.cpu_limit",
		"k8s.container.memory_request",
		"k8s.container.memory_limit",
	}

	ContainerMetricsResourceAttributes = []string{
		"k8s.cluster.name",
		"k8s.container.name",
		"k8s.deployment.name",
		"k8s.namespace.name",
		"k8s.node.name",
		"k8s.pod.name",
		"k8s.pod.uid",
		"service.name",
	}
)
