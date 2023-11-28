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
