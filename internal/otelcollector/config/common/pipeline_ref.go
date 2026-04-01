package common

// PipelineRef identifies a pipeline for component ID and environment variable generation.
type PipelineRef struct {
	Name       string
	SignalType SignalType
}

// typePrefix returns "<signalType>pipeline" when SignalType is set, or an empty string otherwise.
// Example: SignalType="trace" → "tracepipeline", SignalType="" → ""
func (r PipelineRef) typePrefix() string {
	if r.SignalType == "" {
		return ""
	}

	return string(r.SignalType) + "pipeline"
}

// qualifiedName returns the pipeline name, prefixed with the signal type when SignalType is set.
// Example: SignalType="trace", Name="my-pipeline" → "tracepipeline-my-pipeline"
// Example: SignalType="",      Name="my-pipeline" → "my-pipeline"
func (r PipelineRef) qualifiedName() string {
	prefix := r.typePrefix()
	if prefix == "" {
		return r.Name
	}

	return prefix + "-" + r.Name
}
