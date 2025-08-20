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
	// General component IDs
	ComponentIDOTLPReceiver               = "otlp"
	ComponentIDMemoryLimiterProcessor     = "memory_limiter"
	ComponentIDBatchProcessor             = "batch"
	ComponentIDK8sAttributesProcessor     = "k8sattributes"
	ComponentIDIstioNoiseFilterProcessor  = "istio_noise_filter"
	ComponentIDServiceEnrichmentProcessor = "service_enrichment"

	// Specialized component IDs with aliases
	ComponentIDInsertClusterAttributesProcessor = "resource/insert-cluster-attributes"
	ComponentIDDropKymaAttributesProcessor      = "resource/drop-kyma-attributes"

	// Prefixes, shoulbe be used with pipeline names
	ComponentIDPrefixUserDefinedTransformProcessor = "transform/user_defined_"
)
