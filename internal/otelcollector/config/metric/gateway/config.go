package gateway

import (
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
	OTLP                               config.OTLPReceiver                 `yaml:"otlp"`
	SingletonKymaStatsReceiverCreator  *SingletonKymaStatsReceiverCreator  `yaml:"singleton_receiver_creator/kymastats,omitempty"`
	SingletonK8sClusterReceiverCreator *SingletonK8sClusterReceiverCreator `yaml:"singleton_receiver_creator/k8s_cluster,omitempty"`
}

type SingletonKymaStatsReceiverCreator struct {
	AuthType                   string                     `yaml:"auth_type"`
	LeaderElection             LeaderElection             `yaml:"leader_election"`
	SingletonKymaStatsReceiver SingletonKymaStatsReceiver `yaml:"receiver"`
}

type SingletonK8sClusterReceiverCreator struct {
	AuthType                    string                      `yaml:"auth_type"`
	LeaderElection              LeaderElection              `yaml:"leader_election"`
	SingletonK8sClusterReceiver SingletonK8sClusterReceiver `yaml:"receiver"`
}

type LeaderElection struct {
	LeaseName      string `yaml:"lease_name"`
	LeaseNamespace string `yaml:"lease_namespace"`
}

type SingletonKymaStatsReceiver struct {
	KymaStatsReceiver KymaStatsReceiver `yaml:"kymastats"`
}

type KymaStatsReceiver struct {
	AuthType           string      `yaml:"auth_type"`
	CollectionInterval string      `yaml:"collection_interval"`
	Modules            []ModuleGVR `yaml:"modules"`
}

type SingletonK8sClusterReceiver struct {
	K8sClusterReceiver K8sClusterReceiver `yaml:"k8s_cluster"`
}

type K8sClusterReceiver struct {
	AuthType               string                  `yaml:"auth_type"`
	CollectionInterval     string                  `yaml:"collection_interval"`
	NodeConditionsToReport []string                `yaml:"node_conditions_to_report"`
	Metrics                K8sClusterMetricsConfig `yaml:"metrics"`
}

type MetricConfig struct {
	Enabled bool `yaml:"enabled"`
}

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

type ModuleGVR struct {
	Group    string `yaml:"group"`
	Version  string `yaml:"version"`
	Resource string `yaml:"resource"`
}

type Processors struct {
	config.BaseProcessors `yaml:",inline"`

	K8sAttributes                                *config.K8sAttributesProcessor `yaml:"k8sattributes,omitempty"`
	InsertClusterName                            *config.ResourceProcessor      `yaml:"resource/insert-cluster-name,omitempty"`
	DropDiagnosticMetricsIfInputSourcePrometheus *FilterProcessor               `yaml:"filter/drop-diagnostic-metrics-if-input-source-prometheus,omitempty"`
	DropDiagnosticMetricsIfInputSourceIstio      *FilterProcessor               `yaml:"filter/drop-diagnostic-metrics-if-input-source-istio,omitempty"`
	DropIfInputSourceRuntime                     *FilterProcessor               `yaml:"filter/drop-if-input-source-runtime,omitempty"`
	DropIfInputSourcePrometheus                  *FilterProcessor               `yaml:"filter/drop-if-input-source-prometheus,omitempty"`
	DropIfInputSourceIstio                       *FilterProcessor               `yaml:"filter/drop-if-input-source-istio,omitempty"`
	DropIfInputSourceOtlp                        *FilterProcessor               `yaml:"filter/drop-if-input-source-otlp,omitempty"`
	DropRuntimePodMetrics                        *FilterProcessor               `yaml:"filter/drop-runtime-pod-metrics,omitempty"`
	DropRuntimeContainerMetrics                  *FilterProcessor               `yaml:"filter/drop-runtime-container-metrics,omitempty"`
	DropK8sClusterMetrics                        *FilterProcessor               `yaml:"filter/drop-k8s-cluster-metrics,omitempty"`
	ResolveServiceName                           *metric.TransformProcessor     `yaml:"transform/resolve-service-name,omitempty"`
	SetInstrumentationScopeKyma                  *metric.TransformProcessor     `yaml:"transform/set-instrumentation-scope-kyma,omitempty"`
	SetInstrumentationScopeRuntime               *metric.TransformProcessor     `yaml:"transform/set-instrumentation-scope-runtime,omitempty"`

	// NamespaceFilters contains filter processors, which need different configurations per pipeline
	NamespaceFilters NamespaceFilters `yaml:",inline,omitempty"`
}

type NamespaceFilters map[string]*FilterProcessor

type FilterProcessor struct {
	Metrics FilterProcessorMetrics `yaml:"metrics"`
}

type FilterProcessorMetrics struct {
	Metric []string `yaml:"metric,omitempty"`
}

type Exporters map[string]Exporter

type Exporter struct {
	OTLP *config.OTLPExporter `yaml:",inline,omitempty"`
}
