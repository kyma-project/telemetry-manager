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
	OTLP                              config.OTLPReceiver                 `yaml:"otlp"`
	SingletonKymaStatsReceiverCreator *SingletonKymaStatsReceiverCreator  `yaml:"singleton_receiver_creator/kymastats,omitempty"`
	SingletonK8sClusterReceiver       *SingletonK8sClusterReceiverCreator `yaml:"singleton_receiver_creator/k8s_cluster,omitempty"`
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
	AuthType                string   `yaml:"auth_type"`
	CollectionInterval      string   `yaml:"collection_interval"`
	AllocatableTypeToReport []string `yaml:"allocatable_type_to_report"`
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
	ResolveServiceName                           *TransformProcessor            `yaml:"transform/resolve-service-name,omitempty"`
	SetInstrumentationScopeKyma                  *metric.TransformProcessor     `yaml:"transform/set-instrumentation-scope-kyma,omitempty"`
	SetInstrumentationScopeK8sCluster            *metric.TransformProcessor     `yaml:"transform/set-instrumentation-scope-k8s_cluster,omitempty"`

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

type TransformProcessor struct {
	ErrorMode        string                                `yaml:"error_mode"`
	MetricStatements []config.TransformProcessorStatements `yaml:"metric_statements"`
}

type Exporters map[string]Exporter

type Exporter struct {
	OTLP *config.OTLPExporter `yaml:",inline,omitempty"`
}
