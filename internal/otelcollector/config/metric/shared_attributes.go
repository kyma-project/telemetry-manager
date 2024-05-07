package metric

type InputSourceType string

const (
	InputSourceRuntime    InputSourceType = "runtime"
	InputSourcePrometheus InputSourceType = "prometheus"
	InputSourceIstio      InputSourceType = "istio"
	InputSourceOtlp       InputSourceType = "otlp"
)

const (
	TransformedInstrumentationScopeRuntime    = "io.kyma-project.telemetry/runtime"
	TransformedInstrumentationScopePrometheus = "io.kyma-project.telemetry/prometheus"
	TransformedInstrumentationScopeIstio      = "io.kyma-project.telemetry/istio"
)

var TransformedInstrumentationScope = map[InputSourceType]string{
	InputSourceRuntime:    TransformedInstrumentationScopeRuntime,
	InputSourcePrometheus: TransformedInstrumentationScopePrometheus,
	InputSourceIstio:      TransformedInstrumentationScopeIstio,
}

var UpstreamInstrumentationScopeName = map[InputSourceType]string{
	InputSourceRuntime:    "otelcol/kubeletstatsreceiver",
	InputSourcePrometheus: "otelcol/prometheusreceiver",
	InputSourceIstio:      "otelcol/prometheusreceiver",
}
