package runtime

import "slices"

var DefaultMetricsNames = slices.Concat(
	ContainerMetricsNames,
	PodMetricsNames,
	VolumeMetricsNames,
	NodeMetricsNames,
	DeploymentMetricsNames,
	DaemonSetMetricsNames,
	StatefulSetMetricsNames,
	JobsMetricsNames,
)
