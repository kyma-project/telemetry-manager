package gateway

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

type Config struct {
	config.Base `yaml:",inline"`

	Receivers  Receivers  `yaml:"receivers"`
	Processors Processors `yaml:"processors"`
	Exporters  Exporters  `yaml:"exporters"`
	Connectors Connectors `yaml:"connectors"`
}

type Receivers struct {
	OTLP              config.OTLPReceiver `yaml:"otlp"`
	KymaStatsReceiver *KymaStatsReceiver  `yaml:"kymastats,omitempty"`
}

type KymaStatsReceiver struct {
	AuthType           string      `yaml:"auth_type"`
	CollectionInterval string      `yaml:"collection_interval"`
	Resources          []ModuleGVR `yaml:"resources"`
	K8sLeaderElector   string      `yaml:"k8s_leader_elector"`
}

type MetricConfig struct {
	Enabled bool `yaml:"enabled"`
}

type ModuleGVR struct {
	Group    string `yaml:"group"`
	Version  string `yaml:"version"`
	Resource string `yaml:"resource"`
}

type Processors struct {
	config.BaseProcessors `yaml:",inline"`

	// OTel Collector components with static IDs
	K8sAttributes                                *config.K8sAttributesProcessor     `yaml:"k8sattributes,omitempty"`
	InsertClusterAttributes                      *config.ResourceProcessor          `yaml:"resource/insert-cluster-attributes,omitempty"`
	DropDiagnosticMetricsIfInputSourcePrometheus *FilterProcessor                   `yaml:"filter/drop-diagnostic-metrics-if-input-source-prometheus,omitempty"`
	DropDiagnosticMetricsIfInputSourceIstio      *FilterProcessor                   `yaml:"filter/drop-diagnostic-metrics-if-input-source-istio,omitempty"`
	DropIfInputSourceRuntime                     *FilterProcessor                   `yaml:"filter/drop-if-input-source-runtime,omitempty"`
	DropIfInputSourcePrometheus                  *FilterProcessor                   `yaml:"filter/drop-if-input-source-prometheus,omitempty"`
	DropIfInputSourceIstio                       *FilterProcessor                   `yaml:"filter/drop-if-input-source-istio,omitempty"`
	DropIfEnvoyMetricsDisabled                   *FilterProcessor                   `yaml:"filter/drop-envoy-metrics-if-disabled,omitempty"`
	DropIfInputSourceOTLP                        *FilterProcessor                   `yaml:"filter/drop-if-input-source-otlp,omitempty"`
	DropRuntimePodMetrics                        *FilterProcessor                   `yaml:"filter/drop-runtime-pod-metrics,omitempty"`
	DropRuntimeContainerMetrics                  *FilterProcessor                   `yaml:"filter/drop-runtime-container-metrics,omitempty"`
	DropRuntimeNodeMetrics                       *FilterProcessor                   `yaml:"filter/drop-runtime-node-metrics,omitempty"`
	DropRuntimeVolumeMetrics                     *FilterProcessor                   `yaml:"filter/drop-runtime-volume-metrics,omitempty"`
	DropRuntimeDeploymentMetrics                 *FilterProcessor                   `yaml:"filter/drop-runtime-deployment-metrics,omitempty"`
	DropRuntimeStatefulSetMetrics                *FilterProcessor                   `yaml:"filter/drop-runtime-statefulset-metrics,omitempty"`
	DropRuntimeDaemonSetMetrics                  *FilterProcessor                   `yaml:"filter/drop-runtime-daemonset-metrics,omitempty"`
	DropRuntimeJobMetrics                        *FilterProcessor                   `yaml:"filter/drop-runtime-job-metrics,omitempty"`
	ResolveServiceName                           *config.ServiceEnrichmentProcessor `yaml:"service_enrichment,omitempty"`
	DropKymaAttributes                           *config.ResourceProcessor          `yaml:"resource/drop-kyma-attributes,omitempty"`
	SetInstrumentationScopeKyma                  *config.TransformProcessor         `yaml:"transform/set-instrumentation-scope-kyma,omitempty"`
	DeleteSkipEnrichmentAttribute                *config.ResourceProcessor          `yaml:"resource/delete-skip-enrichment-attribute,omitempty"`

	// OTel Collector components with dynamic IDs that are pipeline name based
	NamespaceFilters map[string]*FilterProcessor           `yaml:",inline,omitempty"`
	Transforms       map[string]*config.TransformProcessor `yaml:",inline,omitempty"`
}

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

// Connectors is a map of connectors. The key is the name of the connector. The value is the connector configuration.
// We need to have a different connector per pipeline, so we need to have a map of connectors.
// The value needs to be "any" to satisfy different types of connectors.
type Connectors map[string]any

type RoutingConnector struct {
	DefaultPipelines []string                     `yaml:"default_pipelines"`
	ErrorMode        string                       `yaml:"error_mode"`
	Table            []RoutingConnectorTableEntry `yaml:"table"`
}

type RoutingConnectorTableEntry struct {
	Statement string   `yaml:"statement"`
	Pipelines []string `yaml:"pipelines"`
}
