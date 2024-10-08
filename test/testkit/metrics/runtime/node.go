package runtime

var (
	NodeMetricsNames = kubeletstatsNodeMetricsNames

	kubeletstatsNodeMetricsNames = []string{
		"k8s.node.cpu.usage",
		"k8s.node.filesystem.available",
		"k8s.node.filesystem.capacity",
		"k8s.node.filesystem.usage",
		"k8s.node.memory.available",
		"k8s.node.memory.usage",
		"k8s.node.network.errors",
		"k8s.node.network.io",
		"k8s.node.memory.rss",
		"k8s.node.memory.working_set",
	}

	NodeMetricsResourceAttributes = []string{
		"k8s.cluster.name",
		"k8s.node.name",
	}

	NodeMetricsAttributes = map[string][]string{
		"k8s.node.network.errors": {"interface", "direction"},
		"k8s.node.network.io":     {"interface", "direction"},
	}
)
