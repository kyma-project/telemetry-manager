package runtime

var (
	VolumeMetricsNames = kubeletstatsVolumeMetricsNames

	kubeletstatsVolumeMetricsNames = []string{
		"k8s.volume.available",
		"k8s.volume.capacity",
		"k8s.volume.inodes",
		"k8s.volume.inodes.free",
		"k8s.volume.inodes.used",
	}

	VolumeMetricsResourceAttributes = []string{
		"k8s.cluster.name",
		"k8s.namespace.name",
		"k8s.node.name",
		"k8s.pod.name",
		"k8s.pod.uid",
		"service.name",
		"k8s.persistentvolumeclaim.name",
		"k8s.volume.name",
		"k8s.volume.type",
	}
)
