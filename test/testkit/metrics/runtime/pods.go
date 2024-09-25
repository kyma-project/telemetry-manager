package runtime

import "slices"

var (
	PodMetricsNames = slices.Concat(kubeletstatsPodMetricsNames, k8sclusterPodMetricsNames)

	kubeletstatsPodMetricsNames = []string{
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

	k8sclusterPodMetricsNames = []string{
		"k8s.pod.phase",
	}

	PodMetricsResourceAttributes = []string{
		"k8s.cluster.name",
		"k8s.deployment.name",
		"k8s.namespace.name",
		"k8s.node.name",
		"k8s.pod.name",
		"k8s.pod.uid",
		"kyma.app_name",
		"service.name",
	}

	PodMetricsAttributes = map[string][]string{
		"k8s.pod.network.errors": {"interface", "direction"},
		"k8s.pod.network.io":     {"interface", "direction"},
	}
)
