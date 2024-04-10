package alertrules

type pipelineType string

const (
	typeMetricPipeline pipelineType = "MetricPipeline"
	typeTracePipeline  pipelineType = "TracePipeline"
	typeLogPipeline    pipelineType = "LogPipeline"
)
