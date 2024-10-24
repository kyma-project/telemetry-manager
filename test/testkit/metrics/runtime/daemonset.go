package runtime

var (
	DaemonSetMetricsNames           = k8sclusterDaemonsetMetricsNames
	k8sclusterDaemonsetMetricsNames = []string{
		"k8s.daemonset.current_scheduled_nodes",
		"k8s.daemonset.desired_scheduled_nodes",
		"k8s.daemonset.misscheduled_nodes",
		"k8s.daemonset.ready_nodes",
	}

	DaemonSetResourceAttributes = []string{
		"k8s.namespace.name",
		"k8s.daemonset.name",
		"k8s.daemonset.uid",
		"k8s.cluster.name",
	}
)
