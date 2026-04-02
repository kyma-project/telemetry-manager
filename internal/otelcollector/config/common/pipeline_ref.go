package common

// PipelineRef identifies a pipeline for component ID and environment variable generation.
type PipelineRef struct {
	Name string
	Type SignalType
}

// typePrefix returns "<signalType>pipeline" when Type is set, or an empty string otherwise.
// Example: Type="trace" → "tracepipeline"
// Example: Type=""      → ""
func (r PipelineRef) typePrefix() string {
	if r.Type == "" {
		return ""
	}

	return string(r.Type) + "pipeline"
}

// qualifiedName returns the pipeline name, prefixed with the signal type when Type is set.
// Example: Type="trace", Name="my-pipeline" → "tracepipeline-my-pipeline"
// Example: Type="",      Name="my-pipeline" → "my-pipeline"
func (r PipelineRef) qualifiedName() string {
	prefix := r.typePrefix()
	if prefix == "" {
		return r.Name
	}

	return prefix + "-" + r.Name
}
