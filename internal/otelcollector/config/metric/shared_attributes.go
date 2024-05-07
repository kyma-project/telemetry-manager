package metric

const (
	InputSourceAttribute = "kyma.source"
)

type InputSourceType string

const (
	InputSourceRuntime    InputSourceType = "runtime"
	InputSourcePrometheus InputSourceType = "prometheus"
	InputSourceIstio      InputSourceType = "istio"
	InputSourceOtlp       InputSourceType = "otlp"
)

const (
	InstrumentationScopeRuntime    = "io.kyma-project.telemetry/runtime"
	InstrumentationScopePrometheus = "io.kyma-project.telemetry/prometheus"
	InstrumentationScopeIstio      = "io.kyma-project.telemetry/istio"
)

var InstrumentationScope = map[InputSourceType]string{
	InputSourceRuntime:    InstrumentationScopeRuntime,
	InputSourcePrometheus: InstrumentationScopePrometheus,
	InputSourceIstio:      InstrumentationScopeIstio,
}
