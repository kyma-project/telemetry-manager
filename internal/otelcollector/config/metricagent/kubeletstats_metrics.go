package metricagent

// Kubeletstats receiver metric name constants.
// Source: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/kubeletstatsreceiver/documentation.md
const (
	// Default metrics
	metricContainerCPUTime              = "container.cpu.time"
	metricContainerCPUUsage             = "container.cpu.usage"
	metricContainerFSAvailable          = "container.filesystem.available"
	metricContainerFSCapacity           = "container.filesystem.capacity"
	metricContainerFSUsage              = "container.filesystem.usage"
	metricContainerMemoryAvailable      = "container.memory.available"
	metricContainerMemoryMajorPageFault = "container.memory.major_page_faults"
	metricContainerMemoryPageFaults     = "container.memory.page_faults"
	metricContainerMemoryRSS            = "container.memory.rss"
	metricContainerMemoryUsage          = "container.memory.usage"
	metricContainerMemoryWorkingSet     = "container.memory.working_set"
	metricK8sNodeCPUTime                = "k8s.node.cpu.time"
	metricK8sNodeCPUUsage               = "k8s.node.cpu.usage"
	metricK8sNodeFSAvailable            = "k8s.node.filesystem.available"
	metricK8sNodeFSCapacity             = "k8s.node.filesystem.capacity"
	metricK8sNodeFSUsage                = "k8s.node.filesystem.usage"
	metricK8sNodeMemoryAvailable        = "k8s.node.memory.available"
	metricK8sNodeMemoryMajorPageFaults  = "k8s.node.memory.major_page_faults"
	metricK8sNodeMemoryPageFaults       = "k8s.node.memory.page_faults"
	metricK8sNodeMemoryRSS              = "k8s.node.memory.rss"
	metricK8sNodeMemoryUsage            = "k8s.node.memory.usage"
	metricK8sNodeMemoryWorkingSet       = "k8s.node.memory.working_set"
	metricK8sNodeNetworkErrors          = "k8s.node.network.errors"
	metricK8sNodeNetworkIO              = "k8s.node.network.io"
	metricK8sPodCPUTime                 = "k8s.pod.cpu.time"
	metricK8sPodCPUUsage                = "k8s.pod.cpu.usage"
	metricK8sPodFSAvailable             = "k8s.pod.filesystem.available"
	metricK8sPodFSCapacity              = "k8s.pod.filesystem.capacity"
	metricK8sPodFSUsage                 = "k8s.pod.filesystem.usage"
	metricK8sPodMemoryAvailable         = "k8s.pod.memory.available"
	metricK8sPodMemoryMajorPageFaults   = "k8s.pod.memory.major_page_faults"
	metricK8sPodMemoryPageFaults        = "k8s.pod.memory.page_faults"
	metricK8sPodMemoryRSS               = "k8s.pod.memory.rss"
	metricK8sPodMemoryUsage             = "k8s.pod.memory.usage"
	metricK8sPodMemoryWorkingSet        = "k8s.pod.memory.working_set"
	metricK8sPodNetworkErrors           = "k8s.pod.network.errors"
	metricK8sPodNetworkIO               = "k8s.pod.network.io"
	metricK8sVolumeAvailable            = "k8s.volume.available"
	metricK8sVolumeCapacity             = "k8s.volume.capacity"
	metricK8sVolumeInodes               = "k8s.volume.inodes"
	metricK8sVolumeInodesFree           = "k8s.volume.inodes.free"
	metricK8sVolumeInodesUsed           = "k8s.volume.inodes.used"

	// Optional metrics
	metricContainerUptime                   = "container.uptime"
	metricK8sContainerCPUNodeUtilization    = "k8s.container.cpu.node.utilization"
	metricK8sContainerCPULimitUtilization   = "k8s.container.cpu_limit_utilization"
	metricK8sContainerCPURequestUtilization = "k8s.container.cpu_request_utilization"
	metricK8sContainerMemNodeUtilization    = "k8s.container.memory.node.utilization"
	metricK8sContainerMemLimitUtilization   = "k8s.container.memory_limit_utilization"
	metricK8sContainerMemRequestUtilization = "k8s.container.memory_request_utilization"
	metricK8sNodeSysContainerCPUTime        = "k8s.node.system_container.cpu.time"
	metricK8sNodeSysContainerCPUUsage       = "k8s.node.system_container.cpu.usage"
	metricK8sNodeSysContainerMemUsage       = "k8s.node.system_container.memory.usage"
	metricK8sNodeSysContainerMemWorkingSet  = "k8s.node.system_container.memory.working_set"
	metricK8sNodeUptime                     = "k8s.node.uptime"
	metricK8sPodCPUNodeUtilization          = "k8s.pod.cpu.node.utilization"
	metricK8sPodCPULimitUtilization         = "k8s.pod.cpu_limit_utilization"
	metricK8sPodCPURequestUtilization       = "k8s.pod.cpu_request_utilization"
	metricK8sPodMemNodeUtilization          = "k8s.pod.memory.node.utilization"
	metricK8sPodMemLimitUtilization         = "k8s.pod.memory_limit_utilization"
	metricK8sPodMemRequestUtilization       = "k8s.pod.memory_request_utilization"
	metricK8sPodUptime                      = "k8s.pod.uptime"
	metricK8sPodVolumeUsage                 = "k8s.pod.volume.usage"
)

// KubeletStatsReceiverMetrics contains all metric names that can be emitted by the kubeletstats receiver.
var KubeletStatsReceiverMetrics = []string{
	// Default metrics
	metricContainerCPUTime,
	metricContainerCPUUsage,
	metricContainerFSAvailable,
	metricContainerFSCapacity,
	metricContainerFSUsage,
	metricContainerMemoryAvailable,
	metricContainerMemoryMajorPageFault,
	metricContainerMemoryPageFaults,
	metricContainerMemoryRSS,
	metricContainerMemoryUsage,
	metricContainerMemoryWorkingSet,
	metricK8sNodeCPUTime,
	metricK8sNodeCPUUsage,
	metricK8sNodeFSAvailable,
	metricK8sNodeFSCapacity,
	metricK8sNodeFSUsage,
	metricK8sNodeMemoryAvailable,
	metricK8sNodeMemoryMajorPageFaults,
	metricK8sNodeMemoryPageFaults,
	metricK8sNodeMemoryRSS,
	metricK8sNodeMemoryUsage,
	metricK8sNodeMemoryWorkingSet,
	metricK8sNodeNetworkErrors,
	metricK8sNodeNetworkIO,
	metricK8sPodCPUTime,
	metricK8sPodCPUUsage,
	metricK8sPodFSAvailable,
	metricK8sPodFSCapacity,
	metricK8sPodFSUsage,
	metricK8sPodMemoryAvailable,
	metricK8sPodMemoryMajorPageFaults,
	metricK8sPodMemoryPageFaults,
	metricK8sPodMemoryRSS,
	metricK8sPodMemoryUsage,
	metricK8sPodMemoryWorkingSet,
	metricK8sPodNetworkErrors,
	metricK8sPodNetworkIO,
	metricK8sVolumeAvailable,
	metricK8sVolumeCapacity,
	metricK8sVolumeInodes,
	metricK8sVolumeInodesFree,
	metricK8sVolumeInodesUsed,

	// Optional metrics
	metricContainerUptime,
	metricK8sContainerCPUNodeUtilization,
	metricK8sContainerCPULimitUtilization,
	metricK8sContainerCPURequestUtilization,
	metricK8sContainerMemNodeUtilization,
	metricK8sContainerMemLimitUtilization,
	metricK8sContainerMemRequestUtilization,
	metricK8sNodeSysContainerCPUTime,
	metricK8sNodeSysContainerCPUUsage,
	metricK8sNodeSysContainerMemUsage,
	metricK8sNodeSysContainerMemWorkingSet,
	metricK8sNodeUptime,
	metricK8sPodCPUNodeUtilization,
	metricK8sPodCPULimitUtilization,
	metricK8sPodCPURequestUtilization,
	metricK8sPodMemNodeUtilization,
	metricK8sPodMemLimitUtilization,
	metricK8sPodMemRequestUtilization,
	metricK8sPodUptime,
	metricK8sPodVolumeUsage,
}
