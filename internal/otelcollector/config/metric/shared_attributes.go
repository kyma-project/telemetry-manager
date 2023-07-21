package metric

const (
	InputSourceAttribute = "kyma.source"
)

type InputSourceType string

const (
	InputSourceRuntime   InputSourceType = "runtime"
	InputSourceWorkloads InputSourceType = "workloads"
)
