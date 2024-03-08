package kubeletstats

var (
	MetricNames = []string{
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
		"k8s.pod.cpu.time",
		"k8s.pod.cpu.usage",
		"k8s.pod.filesystem.available",
		"k8s.pod.filesystem.capacity",
		"k8s.pod.filesystem.usage",
		"k8s.pod.memory.available",
		"k8s.pod.memory.major_page_faults",
		"k8s.pod.memory.page_faults",
		"k8s.pod.memory.rss",
		"k8s.pod.memory.usage",
		"k8s.pod.memory.working_set",
		"k8s.pod.network.errors",
		"k8s.pod.network.io",
	}

	MetricResourceAttributes = []string{
		"k8s.cluster.name",
		"k8s.container.name",
		"k8s.namespace.name",
		"k8s.node.name",
		"k8s.pod.name",
		"k8s.pod.uid",
	}
)
