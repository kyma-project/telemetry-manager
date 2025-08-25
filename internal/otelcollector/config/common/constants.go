package common

// ================================================================================
// Environment Variables
// ================================================================================

const (
	EnvVarCurrentPodIP    = "MY_POD_IP"
	EnvVarCurrentNodeName = "MY_NODE_NAME"
	EnvVarGoMemLimit      = "GOMEMLIMIT"
)

// ================================================================================
// Signal Types
// ================================================================================

const (
	SignalTypeMetric = "metric"
	SignalTypeTrace  = "trace"
	SignalTypeLog    = "log"
)

// ================================================================================
// Input Sources and Instrumentation Scopes
// ================================================================================

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

// ================================================================================
// Attributes and Labels
// ================================================================================

const (
	SkipEnrichmentAttribute = "io.kyma-project.telemetry.skip_enrichment"
	KymaInputNameAttribute  = "kyma.input.name"
	KymaInputPrometheus     = "prometheus"
)

// Processor constants
const (
	kymaK8sIOAppName                   = "kyma.kubernetes_io_app_name"
	kymaAppName                        = "kyma.app_name"
	defaultTransformProcessorErrorMode = "ignore"
)

// ================================================================================
// Component IDs
// ================================================================================

const (
	// ================================================================================
	// RECEIVERS
	// ================================================================================

	// Generic Receivers
	ComponentIDOTLPReceiver = "otlp"

	// Log-Specific Receivers
	ComponentIDFileLogReceiver = "filelog/%s" // dynamically filled with pipeline name

	// Metric-Specific Receivers
	ComponentIDKymaStatsReceiver = "kymastats"

	// ================================================================================
	// PROCESSORS
	// ================================================================================

	// Generic Basic Processors
	ComponentIDBatchProcessor         = "batch"
	ComponentIDMemoryLimiterProcessor = "memory_limiter"

	// Generic Enrichment Processors
	ComponentIDK8sAttributesProcessor     = "k8sattributes"
	ComponentIDServiceEnrichmentProcessor = "service_enrichment"

	// Generic Transform Processors
	ComponentIDSetInstrumentationScopeKymaProcessor    = "transform/set-instrumentation-scope-kyma"
	ComponentIDSetInstrumentationScopeRuntimeProcessor = "transform/set-instrumentation-scope-runtime"
	ComponentIDUserDefinedTransformProcessor           = "transform/user-defined-%s" // dynamically filled with pipeline name

	// Generic Resource Processors
	ComponentIDInsertClusterAttributesProcessor       = "resource/insert-cluster-attributes"
	ComponentIDDropKymaAttributesProcessor            = "resource/drop-kyma-attributes"
	ComponentIDDeleteSkipEnrichmentAttributeProcessor = "resource/delete-skip-enrichment-attribute"

	// Generic Input Source Filter Processors
	ComponentIDDropIfInputSourceRuntimeProcessor    = "filter/drop-if-input-source-runtime"
	ComponentIDDropIfInputSourcePrometheusProcessor = "filter/drop-if-input-source-prometheus"
	ComponentIDDropIfInputSourceIstioProcessor      = "filter/drop-if-input-source-istio"
	ComponentIDDropIfInputSourceOTLPProcessor       = "filter/drop-if-input-source-otlp"

	// Log-Specific Processors
	ComponentIDNamespaceFilterProcessor       = "filter/%s-filter-by-namespace" // dynamically filled with pipeline name and input source
	ComponentIDSetObservedTimeIfZeroProcessor = "transform/set-observed-time-if-zero"
	ComponentIDIstioEnrichmentProcessor       = "istio_enrichment"
	ComponentIDIstioNoiseFilterProcessor      = "istio_noise_filter"

	// Metric-Specific Processors
	ComponentIDDropEnvoyMetricsIfDisabledProcessor      = "filter/drop-envoy-metrics-if-disabled"
	ComponentIDNamespacePerInputFilterProcessor         = "filter/%s-filter-by-namespace-%s-input" // dynamically filled with pipeline name and input source
	ComponentIDDropRuntimePodMetricsProcessor           = "filter/drop-runtime-pod-metrics"
	ComponentIDDropRuntimeContainerMetricsProcessor     = "filter/drop-runtime-container-metrics"
	ComponentIDDropRuntimeNodeMetricsProcessor          = "filter/drop-runtime-node-metrics"
	ComponentIDDropRuntimeVolumeMetricsProcessor        = "filter/drop-runtime-volume-metrics"
	ComponentIDDropRuntimeDeploymentMetricsProcessor    = "filter/drop-runtime-deployment-metrics"
	ComponentIDDropRuntimeDaemonSetMetricsProcessor     = "filter/drop-runtime-daemonset-metrics"
	ComponentIDDropRuntimeStatefulSetMetricsProcessor   = "filter/drop-runtime-statefulset-metrics"
	ComponentIDDropRuntimeJobMetricsProcessor           = "filter/drop-runtime-job-metrics"
	ComponentIDDropPrometheusDiagnosticMetricsProcessor = "filter/drop-diagnostic-metrics-if-input-source-prometheus"
	ComponentIDDropIstioDiagnosticMetricsProcessor      = "filter/drop-diagnostic-metrics-if-input-source-istio"

	// ================================================================================
	// EXPORTERS
	// ================================================================================

	// Generic Exporters
	ComponentIDOTLPHTTPExporter = "otlphttp/%s" // dynamically filled with pipeline name
	ComponentIDOTLPGRPCExporter = "otlp/%s"     // dynamically filled with pipeline name

	// ================================================================================
	// CONNECTORS
	// ================================================================================

	// Generic Connectors
	ComponentIDForwardConnector = "forward/%s" // dynamically filled with pipeline name
	ComponentIDRoutingConnector = "routing/%s" // dynamically filled with pipeline name

	// ================================================================================
	// EXTENSIONS AND INFRASTRUCTURE
	// ================================================================================

	// K8s Leader Electors
	ComponentIDK8sLeaderElectorKymaStats  = "telemetry-metric-gateway-kymastats"
	ComponentIDK8sLeaderElectorK8sCluster = "telemetry-metric-agent-k8scluster"
)
