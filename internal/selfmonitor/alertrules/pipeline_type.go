package alertrules

type pipelineType string

const (
	metricPipeline pipelineType = "MetricPipeline"
	tracePipeline  pipelineType = "TracePipeline"
	logPipeline    pipelineType = "LogPipeline"
)
