package metricagent

import (
	"time"
)

type KubeletStatsReceiverConfig struct {
	CollectionInterval          string                         `yaml:"collection_interval"`
	AuthType                    string                         `yaml:"auth_type"`
	Endpoint                    string                         `yaml:"endpoint"`
	InsecureSkipVerify          bool                           `yaml:"insecure_skip_verify"`
	MetricGroups                []MetricGroupType              `yaml:"metric_groups"`
	Metrics                     KubeletStatsMetrics            `yaml:"metrics"`
	ResourceAttributes          KubeletStatsResourceAttributes `yaml:"resource_attributes"`
	ExtraMetadataLabels         []string                       `yaml:"extra_metadata_labels,omitempty"`
	CollectAllNetworkInterfaces NetworkInterfacesEnabler       `yaml:"collect_all_network_interfaces"`
}

type KubeletStatsResourceAttributes struct {
	AWSVolumeID            Metric `yaml:"aws.volume.id"`
	FSType                 Metric `yaml:"fs.type"`
	GCEPDName              Metric `yaml:"gce.pd.name"`
	GlusterFSEndpointsName Metric `yaml:"glusterfs.endpoints.name"`
	GlusterFSPath          Metric `yaml:"glusterfs.path"`
	Partition              Metric `yaml:"partition"`
}

type NetworkInterfacesEnabler struct {
	NodeMetrics bool `yaml:"node"`
}

type Metric struct {
	Enabled bool `yaml:"enabled"`
}

type KubeletStatsMetrics struct {
	*KubeletStatsDefaultMetricsToDrop `yaml:",inline,omitempty"`
	*KubeletStatsPodMetrics           `yaml:",inline,omitempty"`
	*KubeletStatsContainerMetrics     `yaml:",inline,omitempty"`
	*KubeletStatsNodeMetrics          `yaml:",inline,omitempty"`
	*KubeletStatsVolumeMetrics        `yaml:",inline,omitempty"`
	*KubeletStatsOptionalMetrics      `yaml:",inline,omitempty"`
}

type KubeletStatsDefaultMetricsToDrop struct {
	K8sNodeCPUTime               Metric `yaml:"k8s.node.cpu.time"`
	K8sNodeMemoryMajorPageFaults Metric `yaml:"k8s.node.memory.major_page_faults"`
	K8sNodeMemoryPageFaults      Metric `yaml:"k8s.node.memory.page_faults"`
}

type KubeletStatsPodMetrics struct {
	K8sPodCPUTime              Metric `yaml:"k8s.pod.cpu.time,omitempty"`
	K8sPodCPUUsage             Metric `yaml:"k8s.pod.cpu.usage,omitempty"`
	K8sPodFSAvailable          Metric `yaml:"k8s.pod.filesystem.available,omitempty"`
	K8sPodFSCapacity           Metric `yaml:"k8s.pod.filesystem.capacity,omitempty"`
	K8sPodFSUsage              Metric `yaml:"k8s.pod.filesystem.usage,omitempty"`
	K8sPodMemoryAvailable      Metric `yaml:"k8s.pod.memory.available,omitempty"`
	K8sPodMemoryMajorPageFault Metric `yaml:"k8s.pod.memory.major_page_faults,omitempty"`
	K8sPodMemoryPageFaults     Metric `yaml:"k8s.pod.memory.page_faults,omitempty"`
	K8sPodMemoryRSS            Metric `yaml:"k8s.pod.memory.rss,omitempty"`
	K8sPodMemoryUsage          Metric `yaml:"k8s.pod.memory.usage,omitempty"`
	K8sPodMemoryWorkingSet     Metric `yaml:"k8s.pod.memory.working_set,omitempty"`
	K8sPodNetworkErrors        Metric `yaml:"k8s.pod.network.errors,omitempty"`
	K8sPodNetworkIO            Metric `yaml:"k8s.pod.network.io,omitempty"`
}

type KubeletStatsContainerMetrics struct {
	ContainerCPUTime              Metric `yaml:"container.cpu.time,omitempty"`
	ContainerCPUUsage             Metric `yaml:"container.cpu.usage,omitempty"`
	ContainerFSAvailable          Metric `yaml:"container.filesystem.available,omitempty"`
	ContainerFSCapacity           Metric `yaml:"container.filesystem.capacity,omitempty"`
	ContainerFSUsage              Metric `yaml:"container.filesystem.usage,omitempty"`
	ContainerMemoryAvailable      Metric `yaml:"container.memory.available,omitempty"`
	ContainerMemoryMajorPageFault Metric `yaml:"container.memory.major_page_faults,omitempty"`
	ContainerMemoryPageFaults     Metric `yaml:"container.memory.page_faults,omitempty"`
	ContainerMemoryRSS            Metric `yaml:"container.memory.rss,omitempty"`
	ContainerMemoryUsage          Metric `yaml:"container.memory.usage,omitempty"`
	ContainerMemoryWorkingSet     Metric `yaml:"container.memory.working_set,omitempty"`
}

type KubeletStatsNodeMetrics struct {
	K8sNodeCPUUsage         Metric `yaml:"k8s.node.cpu.usage,omitempty"`
	K8sNodeFSAvailable      Metric `yaml:"k8s.node.filesystem.available,omitempty"`
	K8sNodeFSCapacity       Metric `yaml:"k8s.node.filesystem.capacity,omitempty"`
	K8sNodeFSUsage          Metric `yaml:"k8s.node.filesystem.usage,omitempty"`
	K8sNodeMemoryAvailable  Metric `yaml:"k8s.node.memory.available,omitempty"`
	K8sNodeMemoryRSS        Metric `yaml:"k8s.node.memory.rss,omitempty"`
	K8sNodeMemoryUsage      Metric `yaml:"k8s.node.memory.usage,omitempty"`
	K8sNodeMemoryWorkingSet Metric `yaml:"k8s.node.memory.working_set,omitempty"`
	K8sNodeNetworkErrors    Metric `yaml:"k8s.node.network.errors,omitempty"`
	K8sNodeNetworkIO        Metric `yaml:"k8s.node.network.io,omitempty"`
}

type KubeletStatsVolumeMetrics struct {
	K8sVolumeAvailable  Metric `yaml:"k8s.volume.available,omitempty"`
	K8sVolumeCapacity   Metric `yaml:"k8s.volume.capacity,omitempty"`
	K8sVolumeInodes     Metric `yaml:"k8s.volume.inodes,omitempty"`
	K8sVolumeInodesFree Metric `yaml:"k8s.volume.inodes.free,omitempty"`
	K8sVolumeInodesUsed Metric `yaml:"k8s.volume.inodes.used,omitempty"`
}

type KubeletStatsOptionalMetrics struct {
	ContainerUptime                   Metric `yaml:"container.uptime,omitempty"`
	K8sContainerCPUNodeUtilization    Metric `yaml:"k8s.container.cpu.node.utilization,omitempty"`
	K8sContainerCPULimitUtilization   Metric `yaml:"k8s.container.cpu_limit_utilization,omitempty"`
	K8sContainerCPURequestUtilization Metric `yaml:"k8s.container.cpu_request_utilization,omitempty"`
	K8sContainerMemNodeUtilization    Metric `yaml:"k8s.container.memory.node.utilization,omitempty"`
	K8sContainerMemLimitUtilization   Metric `yaml:"k8s.container.memory_limit_utilization,omitempty"`
	K8sContainerMemRequestUtilization Metric `yaml:"k8s.container.memory_request_utilization,omitempty"`
	K8sNodeUptime                     Metric `yaml:"k8s.node.uptime,omitempty"`
	K8sPodCPUNodeUtilization          Metric `yaml:"k8s.pod.cpu.node.utilization,omitempty"`
	K8sPodCPULimitUtilization         Metric `yaml:"k8s.pod.cpu_limit_utilization,omitempty"`
	K8sPodCPURequestUtilization       Metric `yaml:"k8s.pod.cpu_request_utilization,omitempty"`
	K8sPodMemNodeUtilization          Metric `yaml:"k8s.pod.memory.node.utilization,omitempty"`
	K8sPodMemLimitUtilization         Metric `yaml:"k8s.pod.memory_limit_utilization,omitempty"`
	K8sPodMemRequestUtilization       Metric `yaml:"k8s.pod.memory_request_utilization,omitempty"`
	K8sPodUptime                      Metric `yaml:"k8s.pod.uptime,omitempty"`
	K8sPodVolumeUsage                 Metric `yaml:"k8s.pod.volume.usage,omitempty"`
}

type MetricGroupType string

const (
	MetricGroupTypeContainer MetricGroupType = "container"
	MetricGroupTypePod       MetricGroupType = "pod"
	MetricGroupTypeNode      MetricGroupType = "node"
	MetricGroupTypeVolume    MetricGroupType = "volume"
)

type K8sClusterMetrics struct {
	*K8sClusterDefaultMetricsToDrop `yaml:",inline,omitempty"`
	*K8sClusterPodMetrics           `yaml:",inline,omitempty"`
	*K8sClusterContainerMetrics     `yaml:",inline,omitempty"`
	*K8sClusterStatefulSetMetrics   `yaml:",inline,omitempty"`
	*K8sClusterJobMetrics           `yaml:",inline,omitempty"`
	*K8sClusterDeploymentMetrics    `yaml:",inline,omitempty"`
	*K8sClusterDaemonSetMetrics     `yaml:",inline,omitempty"`
	*K8sClusterOptionalMetrics      `yaml:",inline,omitempty"`
}

type K8sClusterDefaultMetricsToDrop struct {
	// Disable some Container metrics by default
	K8sContainerStorageRequest          *Metric `yaml:"k8s.container.storage_request"`
	K8sContainerStorageLimit            *Metric `yaml:"k8s.container.storage_limit"`
	K8sContainerEphemeralStorageRequest *Metric `yaml:"k8s.container.ephemeralstorage_request"`
	K8sContainerEphemeralStorageLimit   *Metric `yaml:"k8s.container.ephemeralstorage_limit"`
	K8sContainerReady                   *Metric `yaml:"k8s.container.ready"`
	// Disable Namespace metrics by default
	K8sNamespacePhase *Metric `yaml:"k8s.namespace.phase"`
	// Disable HPA metrics by default
	K8sHPACurrentReplicas *Metric `yaml:"k8s.hpa.current_replicas"`
	K8sHPADesiredReplicas *Metric `yaml:"k8s.hpa.desired_replicas"`
	K8sHPAMinReplicas     *Metric `yaml:"k8s.hpa.min_replicas"`
	K8sHPAMaxReplicas     *Metric `yaml:"k8s.hpa.max_replicas"`
	// Disable ReplicaSet metrics by default
	K8sReplicaSetAvailable *Metric `yaml:"k8s.replicaset.available"`
	K8sReplicaSetDesired   *Metric `yaml:"k8s.replicaset.desired"`
	// Disable Replication Controller metrics by default
	K8sReplicationControllerAvailable *Metric `yaml:"k8s.replication_controller.available"`
	K8sReplicationControllerDesired   *Metric `yaml:"k8s.replication_controller.desired"`
	// Disable Resource Quota metrics by default
	K8sResourceQuotaHardLimit *Metric `yaml:"k8s.resource_quota.hard_limit"`
	K8sResourceQuotaUsed      *Metric `yaml:"k8s.resource_quota.used"`
	// Disable Cronjob metrics by default
	K8sCronJobActiveJobs *Metric `yaml:"k8s.cronjob.active_jobs"`
}

type K8sClusterStatefulSetMetrics struct {
	K8sStatefulSetCurrentPods *Metric `yaml:"k8s.statefulset.current_pods,omitempty"`
	K8sStatefulSetDesiredPods *Metric `yaml:"k8s.statefulset.desired_pods,omitempty"`
	K8sStatefulSetReadyPods   *Metric `yaml:"k8s.statefulset.ready_pods,omitempty"`
	K8sStatefulSetUpdatedPods *Metric `yaml:"k8s.statefulset.updated_pods,omitempty"`
}

type K8sClusterJobMetrics struct {
	K8sJobActivePods            *Metric `yaml:"k8s.job.active_pods,omitempty"`
	K8sJobDesiredSuccessfulPods *Metric `yaml:"k8s.job.desired_successful_pods,omitempty"`
	K8sJobFailedPods            *Metric `yaml:"k8s.job.failed_pods,omitempty"`
	K8sJobMaxParallelPods       *Metric `yaml:"k8s.job.max_parallel_pods,omitempty"`
	K8sJobSuccessfulPods        *Metric `yaml:"k8s.job.successful_pods,omitempty"`
}

type K8sClusterDeploymentMetrics struct {
	K8sDeploymentAvailable *Metric `yaml:"k8s.deployment.available,omitempty"`
	K8sDeploymentDesired   *Metric `yaml:"k8s.deployment.desired,omitempty"`
}

type K8sClusterDaemonSetMetrics struct {
	K8sDaemonSetCurrentScheduledNodes *Metric `yaml:"k8s.daemonset.current_scheduled_nodes,omitempty"`
	K8sDaemonSetDesiredScheduledNodes *Metric `yaml:"k8s.daemonset.desired_scheduled_nodes,omitempty"`
	K8sDaemonSetMisscheduledNodes     *Metric `yaml:"k8s.daemonset.misscheduled_nodes,omitempty"`
	K8sDaemonSetReadyNodes            *Metric `yaml:"k8s.daemonset.ready_nodes,omitempty"`
}

type K8sClusterPodMetrics struct {
	K8sPodPhase *Metric `yaml:"k8s.pod.phase,omitempty"`
}

type K8sClusterContainerMetrics struct {
	K8sContainerCPURequest    *Metric `yaml:"k8s.container.cpu_request,omitempty"`
	K8sContainerCPULimit      *Metric `yaml:"k8s.container.cpu_limit,omitempty"`
	K8sContainerMemoryRequest *Metric `yaml:"k8s.container.memory_request,omitempty"`
	K8sContainerMemoryLimit   *Metric `yaml:"k8s.container.memory_limit,omitempty"`
	K8sContainerRestarts      *Metric `yaml:"k8s.container.restarts,omitempty"`
}

type K8sClusterOptionalMetrics struct {
	K8sContainerStatusReason           *Metric `yaml:"k8s.container.status.reason,omitempty"`
	K8sContainerStatusState            *Metric `yaml:"k8s.container.status.state,omitempty"`
	K8sNodeCondition                   *Metric `yaml:"k8s.node.condition,omitempty"`
	K8sPodStatusReason                 *Metric `yaml:"k8s.pod.status_reason,omitempty"`
	K8sServiceEndpointCount            *Metric `yaml:"k8s.service.endpoint.count,omitempty"`
	K8sServiceLoadBalancerIngressCount *Metric `yaml:"k8s.service.load_balancer.ingress.count,omitempty"`
}

type K8sClusterReceiverConfig struct {
	AuthType               string            `yaml:"auth_type"`
	CollectionInterval     string            `yaml:"collection_interval"`
	NodeConditionsToReport []string          `yaml:"node_conditions_to_report"`
	Metrics                K8sClusterMetrics `yaml:"metrics"`
	K8sLeaderElector       string            `yaml:"k8s_leader_elector"`
}

type PrometheusReceiverConfig struct {
	Prometheus PrometheusScrape `yaml:"config"`
}

type PrometheusScrape struct {
	ScrapeConfigs []Scrape `yaml:"scrape_configs,omitempty"`
}

type Scrape struct {
	JobName              string        `yaml:"job_name"`
	SampleLimit          int           `yaml:"sample_limit,omitempty"`
	BodySizeLimit        string        `yaml:"body_size_limit,omitempty"`
	ScrapeInterval       time.Duration `yaml:"scrape_interval,omitempty"`
	MetricsPath          string        `yaml:"metrics_path,omitempty"`
	RelabelConfigs       []Relabel     `yaml:"relabel_configs,omitempty"`
	MetricRelabelConfigs []Relabel     `yaml:"metric_relabel_configs,omitempty"`

	KubernetesDiscoveryConfigs []KubernetesDiscovery `yaml:"kubernetes_sd_configs,omitempty"`

	TLS *TLS `yaml:"tls_config,omitempty"`
}

type TLS struct {
	CAFile             string `yaml:"ca_file,omitempty"`
	CertFile           string `yaml:"cert_file,omitempty"`
	KeyFile            string `yaml:"key_file,omitempty"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

type KubernetesDiscovery struct {
	Role      Role                   `yaml:"role"`
	Selectors []K8SDiscoverySelector `yaml:"selectors,omitempty"`
}

type Role string

type K8SDiscoverySelector struct {
	Role  Role   `yaml:"role"`
	Field string `yaml:"field"`
}

const (
	RoleEndpoints Role = "endpoints"
	RolePod       Role = "pod"
)

type Relabel struct {
	SourceLabels []string      `yaml:"source_labels,flow,omitempty"`
	Separator    string        `yaml:"separator,omitempty"`
	Regex        string        `yaml:"regex,omitempty"`
	Modulus      uint64        `yaml:"modulus,omitempty"`
	TargetLabel  string        `yaml:"target_label,omitempty"`
	Replacement  string        `yaml:"replacement,omitempty"`
	Action       RelabelAction `yaml:"action,omitempty"`
}

type RelabelAction string

const (
	Replace  RelabelAction = "replace"
	Keep     RelabelAction = "keep"
	Drop     RelabelAction = "drop"
	LabelMap RelabelAction = "labelmap"
)
