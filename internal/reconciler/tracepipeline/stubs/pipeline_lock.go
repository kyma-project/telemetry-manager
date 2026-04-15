package stubs

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PipelineLock struct{}

func NewPipelineLock() *PipelineLock {
	return &PipelineLock{}
}

func (p *PipelineLock) TryAcquireLock(_ context.Context, _ metav1.Object) error {
	return nil
}

func (p *PipelineLock) IsLockHolder(_ context.Context, _ metav1.Object) error {
	return nil
}
