package runtime

var (
	StatefulSetMetricsNames           = k8sClusterStatefulsetMetricsNames
	k8sClusterStatefulsetMetricsNames = []string{
		"k8s.statefulset.current_pods",
		"k8s.statefulset.desired_pods",
		"k8s.statefulset.ready_pods",
		"k8s.statefulset.updated_pods",
	}

	StatefulSetResourceAttributes = []string{
		"k8s.statefulset.name",
		"k8s.namespace.name",
		"k8s.statefulset.uid",
		"k8s.cluster.name",
	}
)
