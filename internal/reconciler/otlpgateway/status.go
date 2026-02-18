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
// It computes the condition once based on the DaemonSet health and applies it to all pipelines.
// If any status update fails, it logs the error but continues with other pipelines to avoid blocking reconciliation.
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
			// Log error but continue with other pipelines
			log.Error(err, "Failed to update GatewayHealthy condition", "pipeline", name)
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
			// Pipeline was deleted, skip silently
			return nil
		}
		return fmt.Errorf("failed to get TracePipeline: %w", err)
	}

	// Skip if pipeline is being deleted
	if pipeline.DeletionTimestamp != nil {
		logf.FromContext(ctx).V(1).Info("Skipping status update for TracePipeline - marked for deletion", "pipeline", pipelineName)
		return nil
	}

	// Set observed generation
	condition.ObservedGeneration = pipeline.Generation

	// Update condition
	meta.SetStatusCondition(&pipeline.Status.Conditions, *condition)

	// Write status
	if err := r.Status().Update(ctx, &pipeline); err != nil {
		if apierrors.IsConflict(err) {
			// Conflict: will be retried by controller-runtime
			logf.FromContext(ctx).V(1).Info("Status update conflict, will be retried", "pipeline", pipelineName)
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
