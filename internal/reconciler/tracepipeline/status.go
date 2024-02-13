package tracepipeline

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *Reconciler) updateStatus(ctx context.Context, pipelineName string, withinPipelineCountLimit bool) error {
	log := logf.FromContext(ctx)

	var pipeline telemetryv1alpha1.TracePipeline
	if err := r.Get(ctx, types.NamespacedName{Name: pipelineName}, &pipeline); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("failed to get TracePipeline: %v", err)
	}

	if pipeline.DeletionTimestamp != nil {
		return nil
	}

	// If one of the conditions has an empty "Status", it means that the old TracePipelineCondition was used when this pipeline was created
	// In this case, the required "Status" and "Message" fields need to be populated with proper values
	if len(pipeline.Status.Conditions) > 0 && pipeline.Status.Conditions[0].Status == "" {
		log.V(1).Info(fmt.Sprintf("Populating missing fields in the Status conditions for %s", pipeline.Name))
		populateMissingConditionFields(pipeline.Status.Conditions, pipeline.Generation)
		updateStatus(ctx, r.Client, &pipeline)
	}

	if !withinPipelineCountLimit {
		pending := newCondition(
			conditions.TypePending,
			conditions.ReasonMaxPipelinesExceeded,
			metav1.ConditionTrue,
			pipeline.Generation,
		)

		if meta.IsStatusConditionTrue(pipeline.Status.Conditions, conditions.TypeRunning) {
			log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s. Removing the Running condition", pipeline.Name, pending.Type))
			meta.RemoveStatusCondition(&pipeline.Status.Conditions, conditions.TypeRunning)
		}

		meta.SetStatusCondition(&pipeline.Status.Conditions, pending)
		return updateStatus(ctx, r.Client, &pipeline)
	}

	referencesNonExistentSecret := secretref.ReferencesNonExistentSecret(ctx, r.Client, &pipeline)
	if referencesNonExistentSecret {
		pending := newCondition(
			conditions.TypePending,
			conditions.ReasonReferencedSecretMissing,
			metav1.ConditionTrue,
			pipeline.Generation,
		)

		if meta.IsStatusConditionTrue(pipeline.Status.Conditions, conditions.TypeRunning) {
			log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s. Removing the Running condition", pipeline.Name, pending.Type))
			meta.RemoveStatusCondition(&pipeline.Status.Conditions, conditions.TypeRunning)
		}

		meta.SetStatusCondition(&pipeline.Status.Conditions, pending)
		return updateStatus(ctx, r.Client, &pipeline)
	}

	gatewayReady, err := r.prober.IsReady(ctx, types.NamespacedName{Name: r.config.Gateway.BaseName, Namespace: r.config.Gateway.Namespace})
	if err != nil {
		return err
	}

	if gatewayReady {
		if meta.IsStatusConditionTrue(pipeline.Status.Conditions, conditions.TypeRunning) {
			return nil
		}

		pending := newCondition(
			conditions.TypePending,
			conditions.ReasonTraceGatewayDeploymentReady,
			metav1.ConditionFalse,
			pipeline.Generation,
		)
		running := newCondition(
			conditions.TypeRunning,
			conditions.ReasonTraceGatewayDeploymentReady,
			metav1.ConditionTrue,
			pipeline.Generation,
		)
		meta.SetStatusCondition(&pipeline.Status.Conditions, pending)
		meta.SetStatusCondition(&pipeline.Status.Conditions, running)
		return updateStatus(ctx, r.Client, &pipeline)
	}

	pending := newCondition(
		conditions.TypePending,
		conditions.ReasonTraceGatewayDeploymentNotReady,
		metav1.ConditionTrue,
		pipeline.Generation,
	)

	if meta.IsStatusConditionTrue(pipeline.Status.Conditions, conditions.TypeRunning) {
		log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s. Removing the Running condition", pipeline.Name, pending.Type))
		meta.RemoveStatusCondition(&pipeline.Status.Conditions, conditions.TypeRunning)
	}
	meta.SetStatusCondition(&pipeline.Status.Conditions, pending)
	return updateStatus(ctx, r.Client, &pipeline)
}

func populateMissingConditionFields(statusConditions []metav1.Condition, generation int64) {
	if len(statusConditions) == 1 {
		statusConditions[0].Status = metav1.ConditionTrue
		statusConditions[0].Message = conditions.CommonMessageFor(statusConditions[0].Reason)
		statusConditions[0].ObservedGeneration = generation
		return
	}

	for i := range statusConditions {
		if statusConditions[i].Type == conditions.TypePending {
			statusConditions[i].Status = metav1.ConditionFalse
			statusConditions[i].Reason = conditions.ReasonTraceGatewayDeploymentReady
		}
		if statusConditions[i].Type == conditions.TypeRunning {
			statusConditions[i].Status = metav1.ConditionTrue

		}
		statusConditions[i].Message = conditions.CommonMessageFor(statusConditions[i].Reason)
		statusConditions[i].ObservedGeneration = generation
	}
}

func newCondition(condType, reason string, status metav1.ConditionStatus, generation int64) metav1.Condition {
	return metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            conditions.CommonMessageFor(reason),
		ObservedGeneration: generation,
	}
}

func updateStatus(ctx context.Context, client client.Client, pipeline *telemetryv1alpha1.TracePipeline) error {
	if err := client.Status().Update(ctx, pipeline); err != nil {
		return fmt.Errorf("failed to update TracePipeline status: %w", err)
	}
	return nil
}
