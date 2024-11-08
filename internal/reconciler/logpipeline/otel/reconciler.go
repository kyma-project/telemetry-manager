package otel

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline"
	pipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/pipelines"
)

var _ logpipeline.LogPipelineReconciler = &Reconciler{}

type Reconciler struct {
	client.Client
	errToMessageConverter commonstatus.ErrorToMessageConverter
}

func New(client client.Client, errToMessageConverter commonstatus.ErrorToMessageConverter) *Reconciler {
	return &Reconciler{
		Client:                client,
		errToMessageConverter: errToMessageConverter,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
	logf.FromContext(ctx).V(1).Info("Reconciling LogPipeline")

	err := r.doReconcile(ctx, pipeline)

	if statusErr := r.updateStatus(ctx, pipeline.Name); statusErr != nil {
		if err != nil {
			err = fmt.Errorf("failed while updating status: %w: %w", statusErr, err)
		} else {
			err = fmt.Errorf("failed to update status: %w", statusErr)
		}
	}

	return err
}

func (r *Reconciler) SupportedOutput() pipelineutils.LogMode {
	return pipelineutils.OTel
}

func (r *Reconciler) doReconcile(ctx context.Context, _ *telemetryv1alpha1.LogPipeline) error {
	log := logf.FromContext(ctx)
	log.Info("Skipping reconciling LogPipeline in OTel mode")

	return nil
}

func (r *Reconciler) updateStatus(ctx context.Context, _ string) error {
	log := logf.FromContext(ctx)
	log.Info("Skipping status update for LogPipeline in OTel mode")

	return nil
}
