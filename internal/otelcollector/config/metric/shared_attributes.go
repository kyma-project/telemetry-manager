package metric

type InputSourceType string

const (
	InputSourceRuntime    InputSourceType = "runtime"
	InputSourcePrometheus InputSourceType = "prometheus"
	InputSourceIstio      InputSourceType = "istio"
	InputSourceOtlp       InputSourceType = "otlp"
	InputSourceKyma       InputSourceType = "kyma"
	InputSourceK8sCluster InputSourceType = "k8s_cluster"
)

const (
	InstrumentationScopeRuntime    = "io.kyma-project.telemetry/runtime"
	InstrumentationScopePrometheus = "io.kyma-project.telemetry/prometheus"
	InstrumentationScopeIstio      = "io.kyma-project.telemetry/istio"
	InstrumentationScopeKyma       = "io.kyma-project.telemetry/kyma"
	InstrumentationScopeK8sCluster = "io.kyma-project.telemetry/k8s_cluster"
)

var InstrumentationScope = map[InputSourceType]string{
	InputSourceRuntime:    InstrumentationScopeRuntime,
	InputSourcePrometheus: InstrumentationScopePrometheus,
	InputSourceIstio:      InstrumentationScopeIstio,
	InputSourceKyma:       InstrumentationScopeKyma,
	InputSourceK8sCluster: InstrumentationScopeK8sCluster,
}
