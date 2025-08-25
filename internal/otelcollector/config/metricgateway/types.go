package metricgateway

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

// type Processors struct {
// 	common.BaseProcessors `yaml:",inline"`

// 	// OTel Collector components with static IDs
// 	K8sAttributes                                *common.K8sAttributesProcessor     `yaml:"k8sattributes,omitempty"`
// 	InsertClusterAttributes                      *common.ResourceProcessor          `yaml:"resource/insert-cluster-attributes,omitempty"`
// 	DropDiagnosticMetricsIfInputSourcePrometheus *FilterProcessor                   `yaml:"filter/drop-diagnostic-metrics-if-input-source-prometheus,omitempty"`
// 	DropDiagnosticMetricsIfInputSourceIstio      *FilterProcessor                   `yaml:"filter/drop-diagnostic-metrics-if-input-source-istio,omitempty"`
// 	DropIfInputSourceRuntime                     *FilterProcessor                   `yaml:"filter/drop-if-input-source-runtime,omitempty"`
// 	DropIfInputSourcePrometheus                  *FilterProcessor                   `yaml:"filter/drop-if-input-source-prometheus,omitempty"`
// 	DropIfInputSourceIstio                       *FilterProcessor                   `yaml:"filter/drop-if-input-source-istio,omitempty"`
// 	DropIfEnvoyMetricsDisabled                   *FilterProcessor                   `yaml:"filter/drop-envoy-metrics-if-disabled,omitempty"`
// 	DropIfInputSourceOTLP                        *FilterProcessor                   `yaml:"filter/drop-if-input-source-otlp,omitempty"`
// 	DropRuntimePodMetrics                        *FilterProcessor                   `yaml:"filter/drop-runtime-pod-metrics,omitempty"`
// 	DropRuntimeContainerMetrics                  *FilterProcessor                   `yaml:"filter/drop-runtime-container-metrics,omitempty"`
// 	DropRuntimeNodeMetrics                       *FilterProcessor                   `yaml:"filter/drop-runtime-node-metrics,omitempty"`
// 	DropRuntimeVolumeMetrics                     *FilterProcessor                   `yaml:"filter/drop-runtime-volume-metrics,omitempty"`
// 	DropRuntimeDeploymentMetrics                 *FilterProcessor                   `yaml:"filter/drop-runtime-deployment-metrics,omitempty"`
// 	DropRuntimeStatefulSetMetrics                *FilterProcessor                   `yaml:"filter/drop-runtime-statefulset-metrics,omitempty"`
// 	DropRuntimeDaemonSetMetrics                  *FilterProcessor                   `yaml:"filter/drop-runtime-daemonset-metrics,omitempty"`
// 	DropRuntimeJobMetrics                        *FilterProcessor                   `yaml:"filter/drop-runtime-job-metrics,omitempty"`
// 	ResolveServiceName                           *common.ServiceEnrichmentProcessor `yaml:"service_enrichment,omitempty"`
// 	DropKymaAttributes                           *common.ResourceProcessor          `yaml:"resource/drop-kyma-attributes,omitempty"`
// 	SetInstrumentationScopeKyma                  *common.TransformProcessor         `yaml:"transform/set-instrumentation-scope-kyma,omitempty"`
// 	DeleteSkipEnrichmentAttribute                *common.ResourceProcessor          `yaml:"resource/delete-skip-enrichment-attribute,omitempty"`

// 	// OTel Collector components with dynamic IDs that are pipeline name based
// 	Dynamic map[string]any `yaml:",inline,omitempty"`
// }

type FilterProcessor struct {
	Metrics FilterProcessorMetrics `yaml:"metrics"`
}

type FilterProcessorMetrics struct {
	Metric []string `yaml:"metric,omitempty"`
}
