package alertrules

type PipelineType string

const (
	MetricPipeline PipelineType = "MetricPipeline"
	TracePipeline  PipelineType = "TracePipeline"
)
