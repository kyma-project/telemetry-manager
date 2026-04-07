package common

import telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"

// PipelineRef identifies a pipeline for component ID and environment variable generation.
type PipelineRef struct {
	name       string
	signalType SignalType
}

func LogPipelineRef(lp *telemetryv1beta1.LogPipeline) PipelineRef {
	return PipelineRef{name: lp.Name, signalType: SignalTypeLog}
}

func MetricPipelineRef(mp *telemetryv1beta1.MetricPipeline) PipelineRef {
	return PipelineRef{name: mp.Name, signalType: SignalTypeMetric}
}

func TracePipelineRef(tp *telemetryv1beta1.TracePipeline) PipelineRef {
	return PipelineRef{name: tp.Name, signalType: SignalTypeTrace}
}

// typePrefix returns "<signalType>pipeline".
// Example: signalType="trace" → "tracepipeline"
func (r PipelineRef) typePrefix() string {
	if r.signalType == "" {
		return ""
	}

	return string(r.signalType) + "pipeline"
}

// qualifiedName returns the pipeline name, prefixed with the signal type.
// Example: signalType="trace", name="my-pipeline" → "tracepipeline-my-pipeline"
func (r PipelineRef) qualifiedName() string {
	prefix := r.typePrefix()
	if prefix == "" {
		return r.name
	}

	return prefix + "-" + r.name
}
