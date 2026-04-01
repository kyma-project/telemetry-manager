package common

// PipelineRef identifies a pipeline for component ID and environment variable generation.
type PipelineRef struct {
	Name          string
	Type          SignalType
	UseTypePrefix bool
}

// typePrefix returns "<signalType>pipeline" when UseTypePrefix is true, or an empty string otherwise.
// Example: UseTypePrefix=true,  SignalType="trace" → "tracepipeline"
// Example: UseTypePrefix=false, SignalType="log"   → ""
func (r PipelineRef) typePrefix() string {
	if !r.UseTypePrefix {
		return ""
	}

	return string(r.Type) + "pipeline"
}

// qualifiedName returns the pipeline name, prefixed with the signal type when UseTypePrefix is true.
// Example: UseTypePrefix=true,  SignalType="trace", Name="my-pipeline" → "tracepipeline-my-pipeline"
// Example: UseTypePrefix=false, SignalType="log",   Name="my-pipeline" → "my-pipeline"
func (r PipelineRef) qualifiedName() string {
	prefix := r.typePrefix()
	if prefix == "" {
		return r.Name
	}

	return prefix + "-" + r.Name
}
