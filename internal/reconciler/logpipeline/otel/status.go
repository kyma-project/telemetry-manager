package otel

import (
	"context"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *Reconciler) updateStatus(ctx context.Context, _ string) error {
	log := logf.FromContext(ctx)
	log.Info("Skipping status update for LogPipeline in OTel mode")

	return nil
}