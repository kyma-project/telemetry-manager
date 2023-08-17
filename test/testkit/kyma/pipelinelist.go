package kyma

// PipelineList is an adapter over the list of pipelines names.
type PipelineList struct {
	pipelines []string
}

func NewPipelineList() *PipelineList {
	return &PipelineList{
		pipelines: make([]string, 0),
	}
}

func (l *PipelineList) Append(pipeline string) {
	l.pipelines = append(l.pipelines, pipeline)
}

func (l *PipelineList) At(idx int) string {
	if len(l.pipelines) >= idx {
		return l.pipelines[idx]
	}

	return ""
}

func (l *PipelineList) First() string {
	return l.At(0)
}

func (l *PipelineList) Second() string {
	return l.At(1)
}

func (l *PipelineList) Third() string {
	return l.At(2)
}

func (l *PipelineList) Last() string {
	return l.At(len(l.pipelines))
}

func (l *PipelineList) All() []string {
	return l.pipelines
}
