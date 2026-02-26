package otlpgateway

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
)

type PipelineNamesOptions struct {
	LogPipelineNames   []string
	TracePipelineNames []string
}

// updateGatewayHealthyConditions updates the GatewayHealthy condition on all referenced pipeline CRs.
func (r *Reconciler) updateGatewayHealthyConditions(ctx context.Context, opts PipelineNamesOptions) error {
	if len(opts.TracePipelineNames) == 0 && len(opts.LogPipelineNames) == 0 {
		return nil
	}

	log := logf.FromContext(ctx)

	// Compute conditions once (shared across all pipelines of the same type)
	traceCondition := r.computeGatewayHealthyCondition(ctx, commonstatus.SignalTypeTraces)
	logCondition := r.computeGatewayHealthyCondition(ctx, commonstatus.SignalTypeOtelLogs)

	var lastError error

	// Update LogPipelines
	for _, name := range opts.LogPipelineNames {
		if err := r.updateLogPipelineCondition(ctx, name, logCondition); err != nil && !apierrors.IsNotFound(err) {
			log.Error(err, "failed to update log pipeline condition", "pipeline", name)
			lastError = err
		}
	}

	// Update TracePipelines
	for _, name := range opts.TracePipelineNames {
		if err := r.updateTracePipelineCondition(ctx, name, traceCondition); err != nil && !apierrors.IsNotFound(err) {
			log.Error(err, "failed to update trace pipeline condition", "pipeline", name)
			lastError = err
		}
	}

	return lastError
}

// updateTracePipelineCondition updates a single TracePipeline's GatewayHealthy condition.
//
//nolint:dupl // Acceptable duplication - generic approach adds complexity without significant benefit
func (r *Reconciler) updateTracePipelineCondition(ctx context.Context, pipelineName string, condition *metav1.Condition) error {
	var pipeline telemetryv1beta1.TracePipeline
	if err := r.Get(ctx, types.NamespacedName{Name: pipelineName}, &pipeline); err != nil {
		if apierrors.IsNotFound(err) {
			return err
		}

		return fmt.Errorf("failed to get trace pipeline: %w", err)
	}

	if pipeline.DeletionTimestamp != nil {
		logf.FromContext(ctx).V(1).Info("skipping status update for trace pipeline marked for deletion", "pipeline", pipelineName)
		return nil
	}

	condition.ObservedGeneration = pipeline.Generation
	meta.SetStatusCondition(&pipeline.Status.Conditions, *condition)

	if err := r.Status().Update(ctx, &pipeline); err != nil {
		if apierrors.IsConflict(err) {
			logf.FromContext(ctx).V(1).Info("status update conflict, will be retried", "pipeline", pipelineName)
			return nil
		}

		return fmt.Errorf("failed to update trace pipeline status: %w", err)
	}

	return nil
}

// updateLogPipelineCondition updates a single LogPipeline's GatewayHealthy condition.
//
//nolint:dupl // Acceptable duplication - generic approach adds complexity without significant benefit
func (r *Reconciler) updateLogPipelineCondition(ctx context.Context, pipelineName string, condition *metav1.Condition) error {
	var pipeline telemetryv1beta1.LogPipeline
	if err := r.Get(ctx, types.NamespacedName{Name: pipelineName}, &pipeline); err != nil {
		if apierrors.IsNotFound(err) {
			return err
		}

		return fmt.Errorf("failed to get log pipeline: %w", err)
	}

	if pipeline.DeletionTimestamp != nil {
		logf.FromContext(ctx).V(1).Info("skipping status update for log pipeline marked for deletion", "pipeline", pipelineName)
		return nil
	}

	condition.ObservedGeneration = pipeline.Generation
	meta.SetStatusCondition(&pipeline.Status.Conditions, *condition)

	if err := r.Status().Update(ctx, &pipeline); err != nil {
		if apierrors.IsConflict(err) {
			logf.FromContext(ctx).V(1).Info("status update conflict, will be retried", "pipeline", pipelineName)
			return nil
		}

		return fmt.Errorf("failed to update log pipeline status: %w", err)
	}

	return nil
}

// computeGatewayHealthyCondition computes the GatewayHealthy condition based on the DaemonSet health.
func (r *Reconciler) computeGatewayHealthyCondition(ctx context.Context, signalType string) *metav1.Condition {
	return commonstatus.GetGatewayHealthyCondition(
		ctx,
		r.gatewayProber,
		types.NamespacedName{
			Name:      names.OTLPGateway,
			Namespace: r.globals.TargetNamespace(),
		},
		r.errToMsgConverter,
		signalType,
	)
}
