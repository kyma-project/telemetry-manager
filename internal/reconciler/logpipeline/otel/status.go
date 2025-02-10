package otel

import (
	"context"
	"errors"
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/validators/endpoint"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
)

func (r *Reconciler) updateStatus(ctx context.Context, pipelineName string) error {
	var pipeline telemetryv1alpha1.LogPipeline
	if err := r.Get(ctx, types.NamespacedName{Name: pipelineName}, &pipeline); err != nil {
		if apierrors.IsNotFound(err) {
			logf.FromContext(ctx).V(1).Info("Skipping status update for LogPipeline - not found")
			return nil
		}

		return fmt.Errorf("failed to get LogPipeline: %w", err)
	}

	if pipeline.DeletionTimestamp != nil {
		logf.FromContext(ctx).V(1).Info("Skipping status update for LogPipeline - marked for deletion")
		return nil
	}

	r.setGatewayHealthyCondition(ctx, &pipeline)
	r.setGatewayConfigGeneratedCondition(ctx, &pipeline)
	r.setAgentHealthyCondition(ctx, &pipeline)

	r.setFlowHealthCondition(ctx, &pipeline)

	if err := r.Status().Update(ctx, &pipeline); err != nil {
		return fmt.Errorf("failed to update LogPipeline status: %w", err)
	}

	return nil
}

func (r *Reconciler) setFlowHealthCondition(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) {
	status, reason := r.evaluateFlowHealthCondition(ctx, pipeline)

	condition := metav1.Condition{
		Type:               conditions.TypeFlowHealthy,
		Status:             status,
		Reason:             reason,
		Message:            conditions.MessageForLogPipeline(reason),
		ObservedGeneration: pipeline.Generation,
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, condition)
}

func (r *Reconciler) evaluateFlowHealthCondition(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) (metav1.ConditionStatus, string) {
	configGeneratedStatus, _, _ := r.evaluateConfigGeneratedCondition(ctx, pipeline)
	if configGeneratedStatus == metav1.ConditionFalse {
		return metav1.ConditionFalse, conditions.ReasonSelfMonConfigNotGenerated
	}

	probeResult, err := r.flowHealthProber.Probe(ctx, pipeline.Name)
	if err != nil {
		logf.FromContext(ctx).Error(err, "Failed to probe flow health")
		return metav1.ConditionUnknown, conditions.ReasonSelfMonProbingFailed
	}

	logf.FromContext(ctx).V(1).Info("Probed flow health", "result", probeResult)

	reason := flowHealthReasonFor(probeResult)
	if probeResult.Healthy {
		return metav1.ConditionTrue, reason
	}

	return metav1.ConditionFalse, reason
}

func flowHealthReasonFor(probeResult prober.OTelPipelineProbeResult) string {
	if probeResult.AllDataDropped {
		return conditions.ReasonSelfMonAllDataDropped
	}

	if probeResult.SomeDataDropped {
		return conditions.ReasonSelfMonSomeDataDropped
	}

	if probeResult.QueueAlmostFull {
		return conditions.ReasonSelfMonBufferFillingUp
	}

	if probeResult.Throttling {
		return conditions.ReasonSelfMonGatewayThrottling
	}

	return conditions.ReasonSelfMonFlowHealthy
}

func (r *Reconciler) setAgentHealthyCondition(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) {
	condition := &metav1.Condition{
		Type:    conditions.TypeAgentHealthy,
		Status:  metav1.ConditionTrue,
		Reason:  conditions.ReasonLogAgentNotRequired,
		Message: conditions.MessageForLogPipeline(conditions.ReasonLogAgentNotRequired),
	}

	if isLogAgentRequired(pipeline) {
		condition = commonstatus.GetAgentHealthyCondition(ctx,
			r.agentProber,
			types.NamespacedName{Name: otelcollector.LogAgentName, Namespace: r.telemetryNamespace},
			r.errToMessageConverter,
			commonstatus.SignalTypeLogs)
	}

	condition.ObservedGeneration = pipeline.Generation
	meta.SetStatusCondition(&pipeline.Status.Conditions, *condition)
}

func (r *Reconciler) setGatewayHealthyCondition(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) {
	condition := commonstatus.GetGatewayHealthyCondition(ctx,
		r.gatewayProber, types.NamespacedName{Name: otelcollector.LogGatewayName, Namespace: r.telemetryNamespace},
		r.errToMessageConverter,
		commonstatus.SignalTypeLogs)
	condition.ObservedGeneration = pipeline.Generation
	meta.SetStatusCondition(&pipeline.Status.Conditions, *condition)
}

func (r *Reconciler) setGatewayConfigGeneratedCondition(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) {
	status, reason, message := r.evaluateConfigGeneratedCondition(ctx, pipeline)

	condition := metav1.Condition{
		Type:               conditions.TypeConfigurationGenerated,
		Status:             status,
		ObservedGeneration: pipeline.Generation,
		Reason:             reason,
		Message:            message,
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, condition)
}

func (r *Reconciler) evaluateConfigGeneratedCondition(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) (status metav1.ConditionStatus, reason string, message string) {
	err := r.pipelineValidator.validate(ctx, pipeline)
	if err == nil {
		return metav1.ConditionTrue, conditions.ReasonGatewayConfigured, conditions.MessageForLogPipeline(conditions.ReasonGatewayConfigured)
	}

	if errors.Is(err, resourcelock.ErrMaxPipelinesExceeded) {
		return metav1.ConditionFalse, conditions.ReasonMaxPipelinesExceeded, conditions.ConvertErrToMsg(err)
	}

	if errors.Is(err, secretref.ErrSecretRefNotFound) || errors.Is(err, secretref.ErrSecretKeyNotFound) || errors.Is(err, secretref.ErrSecretRefMissingFields) {
		return metav1.ConditionFalse, conditions.ReasonReferencedSecretMissing, conditions.ConvertErrToMsg(err)
	}

	if endpoint.IsEndpointInvalidError(err) {
		return metav1.ConditionFalse,
			conditions.ReasonEndpointInvalid,
			fmt.Sprintf(conditions.MessageForLogPipeline(conditions.ReasonEndpointInvalid), err.Error())
	}

	var APIRequestFailed *errortypes.APIRequestFailedError
	if errors.As(err, &APIRequestFailed) {
		return metav1.ConditionFalse, conditions.ReasonValidationFailed, conditions.MessageForLogPipeline(conditions.ReasonValidationFailed)
	}

	return conditions.EvaluateTLSCertCondition(err)
}
