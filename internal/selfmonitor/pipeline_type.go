package selfmonitor

type PipelineType string

const (
	MetricPipeline PipelineType = "Metric"
	TracePipeline  PipelineType = "Trace"
)
