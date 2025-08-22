package common

// Environment variable names
const (
	EnvVarCurrentPodIP    = "MY_POD_IP"
	EnvVarCurrentNodeName = "MY_NODE_NAME"
	EnvVarGoMemLimit      = "GOMEMLIMIT"
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

const (
	SkipEnrichmentAttribute = "io.kyma-project.telemetry.skip_enrichment"
	KymaInputNameAttribute  = "kyma.input.name"
	KymaInputPrometheus     = "prometheus"
)

// Signal type constants
const (
	SignalTypeMetric = "metric"
	SignalTypeTrace  = "trace"
	SignalTypeLog    = "log"
)

// Processor constants
const (
	kymaK8sIOAppName                   = "kyma.kubernetes_io_app_name"
	kymaAppName                        = "kyma.app_name"
	defaultTransformProcessorErrorMode = "ignore"
)

var upstreamInstrumentationScopeName = map[InputSourceType]string{
	InputSourceRuntime:    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver",
	InputSourcePrometheus: "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver",
	InputSourceIstio:      "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver",
	InputSourceKyma:       "github.com/kyma-project/opentelemetry-collector-components/receiver/kymastatsreceiver",
	InputSourceK8sCluster: "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver",
}

// Component IDs

const (
	// Receivers
	ComponentIDOTLPReceiver      = "otlp"
	ComponentIDFileLogReceiver   = "filelog/%s" // dynamically filled with pipeline name
	ComponentIDKymaStatsReceiver = "kymastats"

	// Processors
	ComponentIDMemoryLimiterProcessor           = "memory_limiter"
	ComponentIDBatchProcessor                   = "batch"
	ComponentIDK8sAttributesProcessor           = "k8sattributes"
	ComponentIDServiceEnrichmentProcessor       = "service_enrichment"
	ComponentIDIstioEnrichmentProcessor         = "istio_enrichment"
	ComponentIDIstioNoiseFilterProcessor        = "istio_noise_filter"
	ComponentIDSetObservedTimeIfZeroProcessor   = "transform/set-observed-time-if-zero"
	ComponentIDSetInstrumentationScopeProcessor = "transform/set-instrumentation-scope-runtime"
	ComponentIDUserDefinedTransformProcessor    = "transform/user-defined-%s" // dynamically filled with pipeline name
	ComponentIDDropIfInputSourceOTLPProcessor   = "filter/drop-if-input-source-otlp"
	ComponentIDNamespaceFilterProcessor         = "filter/%s-filter-by-namespace"
	ComponentIDInsertClusterAttributesProcessor = "resource/insert-cluster-attributes"
	ComponentIDDropKymaAttributesProcessor      = "resource/drop-kyma-attributes"

	// Exporters
	ComponentIDOTLPHTTPExporter = "otlphttp/%s" // dynamically filled with pipeline name
	ComponentIDOTLPGRPCExporter = "otlp/%s"     // dynamically filled with pipeline name

	// Connectors
	ComponentIDRoutingConnector = "routing/%s" // dynamically filled with pipeline name
)
