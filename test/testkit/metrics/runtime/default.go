package runtime

import "slices"

var DefaultMetricsNames = slices.Concat(ContainerMetricsNames, PodMetricsNames)
