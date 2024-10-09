package agent

import (
	"time"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
)

type Config struct {
	config.Base `yaml:",inline"`

	Receivers  Receivers  `yaml:"receivers"`
	Processors Processors `yaml:"processors"`
	Exporters  Exporters  `yaml:"exporters"`
}

type Receivers struct {
	KubeletStats                       *KubeletStatsReceiver               `yaml:"kubeletstats,omitempty"`
	PrometheusAppPods                  *PrometheusReceiver                 `yaml:"prometheus/app-pods,omitempty"`
	PrometheusAppServices              *PrometheusReceiver                 `yaml:"prometheus/app-services,omitempty"`
	PrometheusIstio                    *PrometheusReceiver                 `yaml:"prometheus/istio,omitempty"`
	SingletonK8sClusterReceiverCreator *SingletonK8sClusterReceiverCreator `yaml:"singleton_receiver_creator/k8s_cluster,omitempty"`
}

type KubeletStatsReceiver struct {
	CollectionInterval string                    `yaml:"collection_interval"`
	AuthType           string                    `yaml:"auth_type"`
	Endpoint           string                    `yaml:"endpoint"`
	InsecureSkipVerify bool                      `yaml:"insecure_skip_verify"`
	MetricGroups       []MetricGroupType         `yaml:"metric_groups"`
	Metrics            KubeletStatsMetricsConfig `yaml:"metrics"`
}

type MetricConfig struct {
	Enabled bool `yaml:"enabled"`
}

type KubeletStatsMetricsConfig struct {
	ContainerCPUUsage            MetricConfig `yaml:"container.cpu.usage"`
	ContainerCPUUtilization      MetricConfig `yaml:"container.cpu.utilization"`
	K8sPodCPUUsage               MetricConfig `yaml:"k8s.pod.cpu.usage"`
	K8sPodCPUUtilization         MetricConfig `yaml:"k8s.pod.cpu.utilization"`
	K8sNodeCPUUsage              MetricConfig `yaml:"k8s.node.cpu.usage"`
	K8sNodeCPUUtilization        MetricConfig `yaml:"k8s.node.cpu.utilization"`
	K8sNodeCPUTime               MetricConfig `yaml:"k8s.node.cpu.time"`
	K8sNodeMemoryMajorPageFaults MetricConfig `yaml:"k8s.node.memory.major_page_faults"`
	K8sNodeMemoryPageFaults      MetricConfig `yaml:"k8s.node.memory.page_faults"`
	K8sNodeMemoryRSS             MetricConfig `yaml:"k8s.node.memory.rss"`
	K8sNodeMemoryWorkingSet      MetricConfig `yaml:"k8s.node.memory.working_set"`
}

type MetricGroupType string

const (
	MetricGroupTypeContainer MetricGroupType = "container"
	MetricGroupTypePod       MetricGroupType = "pod"
	MetricGroupTypeNode      MetricGroupType = "node"
)

type K8sClusterMetricsConfig struct {
	// metrics allows enabling/disabling scraped metric.
	K8sContainerStorageRequest          MetricConfig `yaml:"k8s.container.storage_request"`
	K8sContainerStorageLimit            MetricConfig `yaml:"k8s.container.storage_limit"`
	K8sContainerEphemeralStorageRequest MetricConfig `yaml:"k8s.container.ephemeralstorage_request"`
	K8sContainerEphemeralStorageLimit   MetricConfig `yaml:"k8s.container.ephemeralstorage_limit"`
	K8sContainerRestarts                MetricConfig `yaml:"k8s.container.restarts"`
	K8sContainerReady                   MetricConfig `yaml:"k8s.container.ready"`
	K8sNamespacePhase                   MetricConfig `yaml:"k8s.namespace.phase"`
	K8sReplicationControllerAvailable   MetricConfig `yaml:"k8s.replication_controller.available"`
	K8sReplicationControllerDesired     MetricConfig `yaml:"k8s.replication_controller.desired"`
}

type K8sClusterReceiver struct {
	AuthType               string                  `yaml:"auth_type"`
	CollectionInterval     string                  `yaml:"collection_interval"`
	NodeConditionsToReport []string                `yaml:"node_conditions_to_report"`
	Metrics                K8sClusterMetricsConfig `yaml:"metrics"`
}

type SingletonK8sClusterReceiver struct {
	K8sClusterReceiver K8sClusterReceiver `yaml:"k8s_cluster"`
}

type SingletonK8sClusterReceiverCreator struct {
	AuthType                    string                      `yaml:"auth_type"`
	LeaderElection              metric.LeaderElection       `yaml:"leader_election"`
	SingletonK8sClusterReceiver SingletonK8sClusterReceiver `yaml:"receiver"`
}

type PrometheusReceiver struct {
	Config PrometheusConfig `yaml:"config"`
}

type PrometheusConfig struct {
	ScrapeConfigs []ScrapeConfig `yaml:"scrape_configs,omitempty"`
}

type ScrapeConfig struct {
	JobName              string          `yaml:"job_name"`
	SampleLimit          int             `yaml:"sample_limit,omitempty"`
	ScrapeInterval       time.Duration   `yaml:"scrape_interval,omitempty"`
	MetricsPath          string          `yaml:"metrics_path,omitempty"`
	RelabelConfigs       []RelabelConfig `yaml:"relabel_configs,omitempty"`
	MetricRelabelConfigs []RelabelConfig `yaml:"metric_relabel_configs,omitempty"`

	StaticDiscoveryConfigs     []StaticDiscoveryConfig     `yaml:"static_configs,omitempty"`
	KubernetesDiscoveryConfigs []KubernetesDiscoveryConfig `yaml:"kubernetes_sd_configs,omitempty"`

	TLSConfig *TLSConfig `yaml:"tls_config,omitempty"`
}

type TLSConfig struct {
	CAFile             string `yaml:"ca_file,omitempty"`
	CertFile           string `yaml:"cert_file,omitempty"`
	KeyFile            string `yaml:"key_file,omitempty"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

type StaticDiscoveryConfig struct {
	Targets []string `yaml:"targets"`
}

type KubernetesDiscoveryConfig struct {
	Role Role `yaml:"role"`
}

type Role string

const (
	RoleEndpoints Role = "endpoints"
	RolePod       Role = "pod"
)

type RelabelConfig struct {
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
	Replace RelabelAction = "replace"
	Keep    RelabelAction = "keep"
	Drop    RelabelAction = "drop"
)

type Processors struct {
	config.BaseProcessors `yaml:",inline"`

	DeleteServiceName                 *config.ResourceProcessor  `yaml:"resource/delete-service-name,omitempty"`
	DropInternalCommunication         *FilterProcessor           `yaml:"filter/drop-internal-communication,omitempty"`
	SetInstrumentationScopeRuntime    *metric.TransformProcessor `yaml:"transform/set-instrumentation-scope-runtime,omitempty"`
	SetInstrumentationScopePrometheus *metric.TransformProcessor `yaml:"transform/set-instrumentation-scope-prometheus,omitempty"`
	SetInstrumentationScopeIstio      *metric.TransformProcessor `yaml:"transform/set-instrumentation-scope-istio,omitempty"`
	InsertSkipEnrichmentAttribute     *metric.TransformProcessor `yaml:"transform/insert-skip-enrichment-attribute,omitempty"`
}

type Exporters struct {
	OTLP config.OTLPExporter `yaml:"otlp"`
}

type FilterProcessor struct {
	Metrics FilterProcessorMetrics `yaml:"metrics"`
}

type FilterProcessorMetrics struct {
	Metric []string `yaml:"metric,omitempty"`
}
