package logpipeline

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
)

func (r *Reconciler) updateStatus(ctx context.Context, pipelineName string) error {
	if err := r.updateStatusUnsupportedMode(ctx, pipelineName); err != nil {
		return err
	}
	return r.updateStatusConditions(ctx, pipelineName)

}

func (r *Reconciler) updateStatusUnsupportedMode(ctx context.Context, pipelineName string) error {
	var pipeline telemetryv1alpha1.LogPipeline
	if err := r.Get(ctx, types.NamespacedName{Name: pipelineName}, &pipeline); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("failed to get LogPipeline: %v", err)
	}

	desiredUnsupportedMode := pipeline.ContainsCustomPlugin()
	if pipeline.Status.UnsupportedMode != desiredUnsupportedMode {
		pipeline.Status.UnsupportedMode = desiredUnsupportedMode
		if err := r.Status().Update(ctx, &pipeline); err != nil {
			return fmt.Errorf("failed to update LogPipeline unsupported mode status: %v", err)
		}
	}

	return nil
}

func (r *Reconciler) updateStatusConditions(ctx context.Context, pipelineName string) error {
	var pipeline telemetryv1alpha1.LogPipeline
	if err := r.Get(ctx, types.NamespacedName{Name: pipelineName}, &pipeline); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("failed to get LogPipeline: %v", err)
	}

	if pipeline.DeletionTimestamp != nil {
		return nil
	}

	if pipeline.Spec.Output.IsLokiDefined() {
		conditions.SetPendingCondition(ctx, &pipeline.Status.Conditions, pipeline.Generation, conditions.ReasonUnsupportedLokiOutput, pipeline.Name, conditions.LogsMessage)
		return updateStatus(ctx, r.Client, &pipeline)
	}

	referencesNonExistentSecret := secretref.ReferencesNonExistentSecret(ctx, r.Client, &pipeline)
	if referencesNonExistentSecret {
		conditions.SetPendingCondition(ctx, &pipeline.Status.Conditions, pipeline.Generation, conditions.ReasonReferencedSecretMissing, pipeline.Name, conditions.LogsMessage)
		return updateStatus(ctx, r.Client, &pipeline)
	}

	fluentBitReady, err := r.prober.IsReady(ctx, r.config.DaemonSet)
	if err != nil {
		return err
	}

	if !fluentBitReady {
		conditions.SetPendingCondition(ctx, &pipeline.Status.Conditions, pipeline.Generation, conditions.ReasonFluentBitDSNotReady, pipeline.Name, conditions.LogsMessage)
		return updateStatus(ctx, r.Client, &pipeline)
	}

	conditions.SetRunningCondition(ctx, &pipeline.Status.Conditions, pipeline.Generation, conditions.ReasonFluentBitDSReady, pipeline.Name, conditions.LogsMessage)
	return updateStatus(ctx, r.Client, &pipeline)
}

func updateStatus(ctx context.Context, client client.Client, pipeline *telemetryv1alpha1.LogPipeline) error {
	if err := client.Status().Update(ctx, pipeline); err != nil {
		return fmt.Errorf("failed to update LogPipeline status: %w", err)
	}
	return nil
}
