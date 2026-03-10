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
	ContainerCPUUsage            Metric `yaml:"container.cpu.usage"`
	K8sPodCPUUsage               Metric `yaml:"k8s.pod.cpu.usage"`
	K8sNodeCPUUsage              Metric `yaml:"k8s.node.cpu.usage"`
	K8sNodeCPUTime               Metric `yaml:"k8s.node.cpu.time"`
	K8sNodeMemoryMajorPageFaults Metric `yaml:"k8s.node.memory.major_page_faults"`
	K8sNodeMemoryPageFaults      Metric `yaml:"k8s.node.memory.page_faults"`
}

type MetricGroupType string

const (
	MetricGroupTypeContainer MetricGroupType = "container"
	MetricGroupTypePod       MetricGroupType = "pod"
	MetricGroupTypeNode      MetricGroupType = "node"
	MetricGroupTypeVolume    MetricGroupType = "volume"
)

type K8sClusterMetricsToDrop struct {
	*K8sClusterDefaultMetricsToDrop     `yaml:",inline,omitempty"`
	*K8sClusterPodMetricsToDrop         `yaml:",inline,omitempty"`
	*K8sClusterContainerMetricsToDrop   `yaml:",inline,omitempty"`
	*K8sClusterStatefulSetMetricsToDrop `yaml:",inline,omitempty"`
	*K8sClusterJobMetricsToDrop         `yaml:",inline,omitempty"`
	*K8sClusterDeploymentMetricsToDrop  `yaml:",inline,omitempty"`
	*K8sClusterDaemonSetMetricsToDrop   `yaml:",inline,omitempty"`
}

type K8sClusterDefaultMetricsToDrop struct {
	// Disable some Container metrics by default
	K8sContainerStorageRequest          Metric `yaml:"k8s.container.storage_request"`
	K8sContainerStorageLimit            Metric `yaml:"k8s.container.storage_limit"`
	K8sContainerEphemeralStorageRequest Metric `yaml:"k8s.container.ephemeralstorage_request"`
	K8sContainerEphemeralStorageLimit   Metric `yaml:"k8s.container.ephemeralstorage_limit"`
	K8sContainerReady                   Metric `yaml:"k8s.container.ready"`
	// Disable Namespace metrics by default
	K8sNamespacePhase Metric `yaml:"k8s.namespace.phase"`
	// Disable HPA metrics by default
	K8sHPACurrentReplicas Metric `yaml:"k8s.hpa.current_replicas"`
	K8sHPADesiredReplicas Metric `yaml:"k8s.hpa.desired_replicas"`
	K8sHPAMinReplicas     Metric `yaml:"k8s.hpa.min_replicas"`
	K8sHPAMaxReplicas     Metric `yaml:"k8s.hpa.max_replicas"`
	// Disable ReplicaSet metrics by default
	K8sReplicaSetAvailable Metric `yaml:"k8s.replicaset.available"`
	K8sReplicaSetDesired   Metric `yaml:"k8s.replicaset.desired"`
	// Disable Replication Controller metrics by default
	K8sReplicationControllerAvailable Metric `yaml:"k8s.replication_controller.available"`
	K8sReplicationControllerDesired   Metric `yaml:"k8s.replication_controller.desired"`
	// Disable Resource Quota metrics by default
	K8sResourceQuotaHardLimit Metric `yaml:"k8s.resource_quota.hard_limit"`
	K8sResourceQuotaUsed      Metric `yaml:"k8s.resource_quota.used"`
	// Disable Cronjob metrics by default
	K8sCronJobActiveJobs Metric `yaml:"k8s.cronjob.active_jobs"`
}

type K8sClusterStatefulSetMetricsToDrop struct {
	K8sStatefulSetCurrentPods Metric `yaml:"k8s.statefulset.current_pods"`
	K8sStatefulSetDesiredPods Metric `yaml:"k8s.statefulset.desired_pods"`
	K8sStatefulSetReadyPods   Metric `yaml:"k8s.statefulset.ready_pods"`
	K8sStatefulSetUpdatedPods Metric `yaml:"k8s.statefulset.updated_pods"`
}

type K8sClusterJobMetricsToDrop struct {
	K8sJobActivePods            Metric `yaml:"k8s.job.active_pods"`
	K8sJobDesiredSuccessfulPods Metric `yaml:"k8s.job.desired_successful_pods"`
	K8sJobFailedPods            Metric `yaml:"k8s.job.failed_pods"`
	K8sJobMaxParallelPods       Metric `yaml:"k8s.job.max_parallel_pods"`
	K8sJobSuccessfulPods        Metric `yaml:"k8s.job.successful_pods"`
}

type K8sClusterDeploymentMetricsToDrop struct {
	K8sDeploymentAvailable Metric `yaml:"k8s.deployment.available"`
	K8sDeploymentDesired   Metric `yaml:"k8s.deployment.desired"`
}

type K8sClusterDaemonSetMetricsToDrop struct {
	K8sDaemonSetCurrentScheduledNodes Metric `yaml:"k8s.daemonset.current_scheduled_nodes"`
	K8sDaemonSetDesiredScheduledNodes Metric `yaml:"k8s.daemonset.desired_scheduled_nodes"`
	K8sDaemonSetMisscheduledNodes     Metric `yaml:"k8s.daemonset.misscheduled_nodes"`
	K8sDaemonSetReadyNodes            Metric `yaml:"k8s.daemonset.ready_nodes"`
}

type K8sClusterPodMetricsToDrop struct {
	K8sPodPhase Metric `yaml:"k8s.pod.phase"`
}

type K8sClusterContainerMetricsToDrop struct {
	K8sContainerCPURequest    Metric `yaml:"k8s.container.cpu_request"`
	K8sContainerCPULimit      Metric `yaml:"k8s.container.cpu_limit"`
	K8sContainerMemoryRequest Metric `yaml:"k8s.container.memory_request"`
	K8sContainerMemoryLimit   Metric `yaml:"k8s.container.memory_limit"`
	K8sContainerRestarts      Metric `yaml:"k8s.container.restarts"`
}

type K8sClusterReceiverConfig struct {
	AuthType               string                  `yaml:"auth_type"`
	CollectionInterval     string                  `yaml:"collection_interval"`
	NodeConditionsToReport []string                `yaml:"node_conditions_to_report"`
	Metrics                K8sClusterMetricsToDrop `yaml:"metrics"`
	K8sLeaderElector       string                  `yaml:"k8s_leader_elector"`
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

	StaticDiscoveryConfigs     []StaticDiscovery     `yaml:"static_configs,omitempty"`
	KubernetesDiscoveryConfigs []KubernetesDiscovery `yaml:"kubernetes_sd_configs,omitempty"`

	TLS *TLS `yaml:"tls_config,omitempty"`
}

type TLS struct {
	CAFile             string `yaml:"ca_file,omitempty"`
	CertFile           string `yaml:"cert_file,omitempty"`
	KeyFile            string `yaml:"key_file,omitempty"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

type StaticDiscovery struct {
	Targets []string `yaml:"targets"`
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
