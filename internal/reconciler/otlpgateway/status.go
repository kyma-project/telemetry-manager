package otlpgateway

import (
	"context"
	"fmt"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// updateGatewayHealthyConditions updates the GatewayHealthy condition on all referenced TracePipeline CRs.
func (r *Reconciler) updateGatewayHealthyConditions(ctx context.Context, pipelineNames []string) error {
	if len(pipelineNames) == 0 {
		return nil
	}

	log := logf.FromContext(ctx)

	// Compute condition once (shared across all pipelines)
	condition := r.computeGatewayHealthyCondition(ctx)

	// Update each TracePipeline
	var lastError error
	for _, name := range pipelineNames {
		if err := r.updatePipelineCondition(ctx, name, condition); err != nil {
			log.Error(err, "failed to update gateway healthy condition", "pipeline", name)
			lastError = err
		}
	}

	return lastError
}

// updatePipelineCondition updates a single TracePipeline's GatewayHealthy condition.
func (r *Reconciler) updatePipelineCondition(ctx context.Context, pipelineName string, condition *metav1.Condition) error {
	var pipeline telemetryv1beta1.TracePipeline
	if err := r.Get(ctx, types.NamespacedName{Name: pipelineName}, &pipeline); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get pipeline: %w", err)
	}

	if pipeline.DeletionTimestamp != nil {
		logf.FromContext(ctx).V(1).Info("skipping status update for pipeline marked for deletion", "pipeline", pipelineName)
		return nil
	}

	condition.ObservedGeneration = pipeline.Generation
	meta.SetStatusCondition(&pipeline.Status.Conditions, *condition)

	if err := r.Status().Update(ctx, &pipeline); err != nil {
		if apierrors.IsConflict(err) {
			logf.FromContext(ctx).V(1).Info("status update conflict, will be retried", "pipeline", pipelineName)
			return nil
		}
		return fmt.Errorf("failed to update status: %w", err)
	}

	return nil
}

// computeGatewayHealthyCondition computes the GatewayHealthy condition based on the DaemonSet health.
func (r *Reconciler) computeGatewayHealthyCondition(ctx context.Context) *metav1.Condition {
	return commonstatus.GetGatewayHealthyCondition(
		ctx,
		r.gatewayProber,
		types.NamespacedName{
			Name:      names.OTLPGateway,
			Namespace: r.globals.TargetNamespace(),
		},
		r.errToMsgConverter,
		commonstatus.SignalTypeTraces,
	)
}
