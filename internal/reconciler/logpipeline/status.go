package logpipeline

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

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

	log := logf.FromContext(ctx)

	if pipeline.Spec.Output.IsLokiDefined() {
		pending := conditions.New(
			conditions.TypePending,
			conditions.ReasonUnsupportedLokiOutput,
			metav1.ConditionTrue,
			pipeline.Generation,
		)

		if meta.FindStatusCondition(pipeline.Status.Conditions, conditions.TypeRunning) != nil {
			log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s. Removing the Running condition", pipeline.Name, pending.Type))
			meta.RemoveStatusCondition(&pipeline.Status.Conditions, conditions.TypeRunning)
		}

		meta.SetStatusCondition(&pipeline.Status.Conditions, pending)
		return updateStatus(ctx, r.Client, &pipeline)
	}

	referencesNonExistentSecret := secretref.ReferencesNonExistentSecret(ctx, r.Client, &pipeline)
	if referencesNonExistentSecret {
		pending := conditions.New(
			conditions.TypePending,
			conditions.ReasonReferencedSecretMissing,
			metav1.ConditionTrue,
			pipeline.Generation,
		)

		if meta.FindStatusCondition(pipeline.Status.Conditions, conditions.TypeRunning) != nil {
			log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s. Removing the Running condition", pipeline.Name, pending.Type))
			meta.RemoveStatusCondition(&pipeline.Status.Conditions, conditions.TypeRunning)
		}

		meta.SetStatusCondition(&pipeline.Status.Conditions, pending)
		return updateStatus(ctx, r.Client, &pipeline)
	}

	fluentBitReady, err := r.prober.IsReady(ctx, r.config.DaemonSet)
	if err != nil {
		return err
	}

	if fluentBitReady {
		existingPending := meta.FindStatusCondition(pipeline.Status.Conditions, conditions.TypePending)
		if existingPending != nil {
			newPending := conditions.New(
				conditions.TypePending,
				existingPending.Reason,
				metav1.ConditionFalse,
				pipeline.Generation,
			)
			meta.SetStatusCondition(&pipeline.Status.Conditions, newPending)
		}

		running := conditions.New(
			conditions.TypeRunning,
			conditions.ReasonFluentBitDSReady,
			metav1.ConditionTrue,
			pipeline.Generation,
		)
		meta.SetStatusCondition(&pipeline.Status.Conditions, running)

		return updateStatus(ctx, r.Client, &pipeline)
	}

	pending := conditions.New(
		conditions.TypePending,
		conditions.ReasonFluentBitDSNotReady,
		metav1.ConditionTrue,
		pipeline.Generation,
	)

	if meta.FindStatusCondition(pipeline.Status.Conditions, conditions.TypeRunning) != nil {
		log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s. Removing the Running condition", pipeline.Name, pending.Type))
		meta.RemoveStatusCondition(&pipeline.Status.Conditions, conditions.TypeRunning)
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, pending)
	return updateStatus(ctx, r.Client, &pipeline)
}

func updateStatus(ctx context.Context, client client.Client, pipeline *telemetryv1alpha1.LogPipeline) error {
	if err := client.Status().Update(ctx, pipeline); err != nil {
		return fmt.Errorf("failed to update LogPipeline status: %w", err)
	}
	return nil
}
