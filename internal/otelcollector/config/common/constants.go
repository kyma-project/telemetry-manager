package common

const (
	EnvVarCurrentPodIP    = "MY_POD_IP"
	EnvVarCurrentNodeName = "MY_NODE_NAME"
	EnvVarGoMemLimit      = "GOMEMLIMIT"
)

const (
	SignalTypeMetric = "metric"
	SignalTypeTrace  = "trace"
	SignalTypeLog    = "log"
)

type InputSourceType string

const (
	InputSourceRuntime    InputSourceType = "runtime"
	InputSourcePrometheus InputSourceType = "prometheus"
	InputSourceIstio      InputSourceType = "istio"
	InputSourceOTLP       InputSourceType = "otlp"
	InputSourceKyma       InputSourceType = "kyma"
	InputSourceK8sCluster InputSourceType = "k8s_cluster"
)

const (
	InstrumentationScopeRuntime    = "io.kyma-project.telemetry/runtime"
	InstrumentationScopePrometheus = "io.kyma-project.telemetry/prometheus"
	InstrumentationScopeIstio      = "io.kyma-project.telemetry/istio"
	InstrumentationScopeKyma       = "io.kyma-project.telemetry/kyma"
)

var InstrumentationScope = map[InputSourceType]string{
	InputSourceRuntime:    InstrumentationScopeRuntime,
	InputSourcePrometheus: InstrumentationScopePrometheus,
	InputSourceIstio:      InstrumentationScopeIstio,
	InputSourceKyma:       InstrumentationScopeKyma,
	InputSourceK8sCluster: InstrumentationScopeRuntime,
}

var upstreamInstrumentationScopeName = map[InputSourceType]string{
	InputSourceRuntime:    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver",
	InputSourcePrometheus: "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver",
	InputSourceIstio:      "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver",
	InputSourceKyma:       "github.com/kyma-project/opentelemetry-collector-components/receiver/kymastatsreceiver",
	InputSourceK8sCluster: "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver",
}

const (
	SkipEnrichmentAttribute = "io.kyma-project.telemetry.skip_enrichment"
	KymaInputNameAttribute  = "kyma.input.name"
	KymaInputPrometheus     = "prometheus"
)

const (
	kymaK8sIOAppName                   = "kyma.kubernetes_io_app_name"
	kymaAppName                        = "kyma.app_name"
	defaultTransformProcessorErrorMode = "ignore"
)

const (
	K8sLeaderElectorKymaStats  = "telemetry-metric-gateway-kymastats"
	K8sLeaderElectorK8sCluster = "telemetry-metric-agent-k8scluster"
)

const (
	MetricsBatchingMaxQueueSize = 256 // Maximum number of batches kept in memory before dropping
)

// ================================================================================
// Component IDs
// ================================================================================

const (
	// ================================================================================
	// RECEIVERS
	// ================================================================================

	ComponentIDOTLPReceiver                  = "otlp"
	ComponentIDFileLogReceiver               = "filelog/%s" // dynamically filled with pipeline name
	ComponentIDKymaStatsReceiver             = "kymastats"
	ComponentIDK8sClusterReceiver            = "k8s_cluster"
	ComponentIDKubeletStatsReceiver          = "kubeletstats"
	ComponentIDPrometheusAppPodsReceiver     = "prometheus/app-pods"
	ComponentIDPrometheusAppServicesReceiver = "prometheus/app-services"
	ComponentIDPrometheusIstioReceiver       = "prometheus/istio"

	// ================================================================================
	// PROCESSORS
	// ================================================================================

	// Common Processors
	ComponentIDBatchProcessor                          = "batch"
	ComponentIDMemoryLimiterProcessor                  = "memory_limiter"
	ComponentIDK8sAttributesProcessor                  = "k8sattributes"
	ComponentIDServiceEnrichmentProcessor              = "service_enrichment"
	ComponentIDIstioNoiseFilterProcessor               = "istio_noise_filter"
	ComponentIDSetInstrumentationScopeKymaProcessor    = "transform/set-instrumentation-scope-kyma"
	ComponentIDSetInstrumentationScopeRuntimeProcessor = "transform/set-instrumentation-scope-runtime"
	ComponentIDUserDefinedTransformProcessor           = "transform/user-defined-%s" // dynamically filled with pipeline name
	ComponentIDInsertClusterAttributesProcessor        = "resource/insert-cluster-attributes"
	ComponentIDDropKymaAttributesProcessor             = "resource/drop-kyma-attributes"

	// Log-Specific Processors
	ComponentIDNamespaceFilterProcessor       = "filter/%s-filter-by-namespace" // dynamically filled with pipeline name and input source
	ComponentIDSetObservedTimeIfZeroProcessor = "transform/set-observed-time-if-zero"
	ComponentIDIstioEnrichmentProcessor       = "istio_enrichment"

	// Metric-Specific Processors
	ComponentIDDropIfInputSourceRuntimeProcessor           = "filter/drop-if-input-source-runtime"
	ComponentIDDropIfInputSourcePrometheusProcessor        = "filter/drop-if-input-source-prometheus"
	ComponentIDDropIfInputSourceIstioProcessor             = "filter/drop-if-input-source-istio"
	ComponentIDDropIfInputSourceOTLPProcessor              = "filter/drop-if-input-source-otlp"
	ComponentIDDropEnvoyMetricsIfDisabledProcessor         = "filter/drop-envoy-metrics-if-disabled"
	ComponentIDNamespacePerInputFilterProcessor            = "filter/%s-filter-by-namespace-%s-input" // dynamically filled with pipeline name and input source
	ComponentIDDropRuntimePodMetricsProcessor              = "filter/drop-runtime-pod-metrics"
	ComponentIDDropRuntimeContainerMetricsProcessor        = "filter/drop-runtime-container-metrics"
	ComponentIDDropRuntimeNodeMetricsProcessor             = "filter/drop-runtime-node-metrics"
	ComponentIDDropRuntimeVolumeMetricsProcessor           = "filter/drop-runtime-volume-metrics"
	ComponentIDDropRuntimeDeploymentMetricsProcessor       = "filter/drop-runtime-deployment-metrics"
	ComponentIDDropRuntimeDaemonSetMetricsProcessor        = "filter/drop-runtime-daemonset-metrics"
	ComponentIDDropRuntimeStatefulSetMetricsProcessor      = "filter/drop-runtime-statefulset-metrics"
	ComponentIDDropRuntimeJobMetricsProcessor              = "filter/drop-runtime-job-metrics"
	ComponentIDDropPrometheusDiagnosticMetricsProcessor    = "filter/drop-diagnostic-metrics-if-input-source-prometheus"
	ComponentIDDropIstioDiagnosticMetricsProcessor         = "filter/drop-diagnostic-metrics-if-input-source-istio"
	ComponentIDFilterDropNonPVCVolumesMetricsProcessor     = "filter/drop-non-pvc-volumes-metrics"
	ComponentIDFilterDropVirtualNetworkInterfacesProcessor = "filter/drop-virtual-network-interfaces"
	ComponentIDResourceDeleteServiceNameProcessor          = "resource/delete-service-name"
	ComponentIDDeleteSkipEnrichmentAttributeProcessor      = "resource/delete-skip-enrichment-attribute"
	ComponentIDSetInstrumentationScopePrometheusProcessor  = "transform/set-instrumentation-scope-prometheus"
	ComponentIDSetInstrumentationScopeIstioProcessor       = "transform/set-instrumentation-scope-istio"
	ComponentIDInsertSkipEnrichmentAttributeProcessor      = "transform/insert-skip-enrichment-attribute"

	// ================================================================================
	// EXPORTERS
	// ================================================================================

	ComponentIDOTLPHTTPExporter = "otlphttp/%s" // dynamically filled with pipeline name
	ComponentIDOTLPGRPCExporter = "otlp/%s"     // dynamically filled with pipeline name
	ComponentIDOTLPExporter     = "otlp"        // static OTLP exporter

	// ================================================================================
	// CONNECTORS
	// ================================================================================

	ComponentIDForwardConnector = "forward/%s" // dynamically filled with pipeline name
	ComponentIDRoutingConnector = "routing/%s" // dynamically filled with pipeline name
	ComponentIDEnrichmentOutputRoutingConnector = "routing/enrichment-output"
	ComponentIDRuntimeInputRoutingConnector    = "routing/runtime-input"
	ComponentIDPrometheusInputRoutingConnector = "routing/prometheus-input"
	ComponentIDIstioInputRoutingConnector      = "routing/istio-input"

	// ================================================================================
	// EXTENSIONS
	// ================================================================================
	ComponentIDK8sLeaderElectorExtension = "k8s_leader_elector"
	ComponentIDFileStorageExtension      = "file_storage"
	ComponentIDHealthCheckExtension      = "health_check"
	ComponentIDPprofExtension            = "pprof"
)
