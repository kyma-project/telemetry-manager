package stubs

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PipelineSync struct{}

func NewPipelineSync() *PipelineSync {
	return &PipelineSync{}
}

func (p *PipelineSync) TryAcquireLock(_ context.Context, _ metav1.Object) error {
	return nil
}
