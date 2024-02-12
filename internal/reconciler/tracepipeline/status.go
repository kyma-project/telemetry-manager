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
		populateMissingConditionFields(ctx, r.Client, &pipeline)
	}

	if !withinPipelineCountLimit {
		pending := newCondition(
			conditions.TypePending,
			conditions.ReasonMaxPipelinesExceeded,
			metav1.ConditionTrue,
			pipeline.Generation,
		)

		if pipeline.Status.HasCondition(conditions.TypeRunning) {
			log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s. Resetting previous conditions", pipeline.Name, pending.Type))
			pipeline.Status.Conditions = []metav1.Condition{}
		}

		return setCondition(ctx, r.Client, &pipeline, pending)
	}

	referencesNonExistentSecret := secretref.ReferencesNonExistentSecret(ctx, r.Client, &pipeline)
	if referencesNonExistentSecret {
		pending := newCondition(
			conditions.TypePending,
			conditions.ReasonReferencedSecretMissing,
			metav1.ConditionTrue,
			pipeline.Generation,
		)

		if pipeline.Status.HasCondition(conditions.TypeRunning) {
			log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s. Resetting previous conditions", pipeline.Name, pending.Type))
			pipeline.Status.Conditions = []metav1.Condition{}
		}

		return setCondition(ctx, r.Client, &pipeline, pending)
	}

	gatewayReady, err := r.prober.IsReady(ctx, types.NamespacedName{Name: r.config.Gateway.BaseName, Namespace: r.config.Gateway.Namespace})
	if err != nil {
		return err
	}

	if gatewayReady {
		if pipeline.Status.HasCondition(conditions.TypeRunning) {
			return nil
		}

		running := newCondition(
			conditions.TypeRunning,
			conditions.ReasonTraceGatewayDeploymentReady,
			metav1.ConditionTrue,
			pipeline.Generation,
		)
		return setCondition(ctx, r.Client, &pipeline, running)
	}

	pending := newCondition(
		conditions.TypePending,
		conditions.ReasonTraceGatewayDeploymentNotReady,
		metav1.ConditionTrue,
		pipeline.Generation,
	)

	if pipeline.Status.HasCondition(conditions.TypeRunning) {
		log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s. Resetting previous conditions", pipeline.Name, pending.Type))
		pipeline.Status.Conditions = []metav1.Condition{}
	}

	return setCondition(ctx, r.Client, &pipeline, pending)
}

func populateMissingConditionFields(ctx context.Context, client client.Client, pipeline *telemetryv1alpha1.TracePipeline) error {
	log := logf.FromContext(ctx)
	log.V(1).Info(fmt.Sprintf("Populating missing fields in the Status conditions for %s", pipeline.Name))

	for i := range pipeline.Status.Conditions {
		pipeline.Status.Conditions[i].Status = metav1.ConditionTrue
		pipeline.Status.Conditions[i].Message = conditions.CommonMessageFor(pipeline.Status.Conditions[i].Reason)
	}

	if err := client.Status().Update(ctx, pipeline); err != nil {
		return fmt.Errorf("failed to update TracePipeline status when poplulating missing fields in conditions: %v", err)
	}
	return nil
}

func newCondition(condType, reason string, status metav1.ConditionStatus, generation int64) *metav1.Condition {
	return &metav1.Condition{
		LastTransitionTime: metav1.Now(),
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            conditions.CommonMessageFor(reason),
		ObservedGeneration: generation,
	}
}

func setCondition(ctx context.Context, client client.Client, pipeline *telemetryv1alpha1.TracePipeline, condition *metav1.Condition) error {
	log := logf.FromContext(ctx)

	log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s", pipeline.Name, condition.Type))

	pipeline.Status.SetCondition(*condition)

	if err := client.Status().Update(ctx, pipeline); err != nil {
		return fmt.Errorf("failed to update TracePipeline status to %s: %v", condition.Type, err)
	}
	return nil
}
