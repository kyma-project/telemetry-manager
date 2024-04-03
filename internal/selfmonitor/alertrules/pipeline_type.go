package alertrules

type PipelineType string

const (
	MetricPipeline PipelineType = "Metric"
	TracePipeline  PipelineType = "Trace"
)
