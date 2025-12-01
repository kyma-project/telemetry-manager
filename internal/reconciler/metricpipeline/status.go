package metricpipeline

import (
	"context"
	"errors"
	"fmt"

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
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	"github.com/kyma-project/telemetry-manager/internal/validators/endpoint"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
)

func (r *Reconciler) updateStatus(ctx context.Context, pipelineName string) error {
	var pipeline telemetryv1alpha1.MetricPipeline
	if err := r.Get(ctx, types.NamespacedName{Name: pipelineName}, &pipeline); err != nil {
		if apierrors.IsNotFound(err) {
			logf.FromContext(ctx).V(1).Info("Skipping status update for MetricPipeline - not found")
			return nil
		}

		return fmt.Errorf("failed to get MetricPipeline: %w", err)
	}

	if pipeline.DeletionTimestamp != nil {
		logf.FromContext(ctx).V(1).Info("Skipping status update for MetricPipeline - marked for deletion")
		return nil
	}

	r.setAgentHealthyCondition(ctx, &pipeline)
	r.setGatewayHealthyCondition(ctx, &pipeline)
	r.setGatewayConfigGeneratedCondition(ctx, &pipeline)
	r.setFlowHealthCondition(ctx, &pipeline)

	if err := r.Status().Update(ctx, &pipeline); err != nil {
		return fmt.Errorf("failed to update MetricPipeline status: %w", err)
	}

	return nil
}

func (r *Reconciler) setAgentHealthyCondition(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline) {
	condition := &metav1.Condition{
		Type:    conditions.TypeAgentHealthy,
		Status:  metav1.ConditionTrue,
		Reason:  conditions.ReasonMetricAgentNotRequired,
		Message: conditions.MessageForMetricPipeline(conditions.ReasonMetricAgentNotRequired),
	}

	if isMetricAgentRequired(pipeline) {
		condition = commonstatus.GetAgentHealthyCondition(ctx,
			r.agentProber,
			types.NamespacedName{Name: otelcollector.MetricAgentName, Namespace: r.globals.TargetNamespace()},
			r.errToMsgConverter,
			commonstatus.SignalTypeMetrics)
	}

	condition.ObservedGeneration = pipeline.Generation
	meta.SetStatusCondition(&pipeline.Status.Conditions, *condition)
}

func (r *Reconciler) setGatewayHealthyCondition(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline) {
	condition := commonstatus.GetGatewayHealthyCondition(ctx,
		r.gatewayProber, types.NamespacedName{Name: otelcollector.MetricGatewayName, Namespace: r.globals.TargetNamespace()},
		r.errToMsgConverter,
		commonstatus.SignalTypeMetrics)
	condition.ObservedGeneration = pipeline.Generation
	meta.SetStatusCondition(&pipeline.Status.Conditions, *condition)
}

func (r *Reconciler) setGatewayConfigGeneratedCondition(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline) {
	status, reason, message := r.evaluateConfigGeneratedCondition(ctx, pipeline)
	condition := metav1.Condition{
		Type:               conditions.TypeConfigurationGenerated,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: pipeline.Generation,
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, condition)
}

func (r *Reconciler) evaluateConfigGeneratedCondition(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline) (status metav1.ConditionStatus, reason string, message string) {
	err := r.pipelineValidator.validate(ctx, pipeline)
	if err == nil {
		return metav1.ConditionTrue, conditions.ReasonGatewayConfigured, conditions.MessageForMetricPipeline(conditions.ReasonGatewayConfigured)
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
			fmt.Sprintf(conditions.MessageForMetricPipeline(conditions.ReasonEndpointInvalid), err.Error())
	}

	if ottl.IsInvalidOTTLSpecError(err) {
		return metav1.ConditionFalse,
			conditions.ReasonOTTLSpecInvalid,
			fmt.Sprintf(conditions.MessageForMetricPipeline(conditions.ReasonOTTLSpecInvalid), err.Error())
	}

	var APIRequestFailed *errortypes.APIRequestFailedError
	if errors.As(err, &APIRequestFailed) {
		return metav1.ConditionFalse, conditions.ReasonValidationFailed, conditions.MessageForMetricPipeline(conditions.ReasonValidationFailed)
	}

	return conditions.EvaluateTLSCertCondition(err)
}

func (r *Reconciler) setFlowHealthCondition(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline) {
	status, reason := r.evaluateFlowHealthCondition(ctx, pipeline)

	condition := metav1.Condition{
		Type:               conditions.TypeFlowHealthy,
		Status:             status,
		Reason:             reason,
		Message:            conditions.MessageForMetricPipeline(reason),
		ObservedGeneration: pipeline.Generation,
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, condition)
}

func (r *Reconciler) evaluateFlowHealthCondition(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline) (metav1.ConditionStatus, string) {
	configGeneratedStatus, _, _ := r.evaluateConfigGeneratedCondition(ctx, pipeline)
	if configGeneratedStatus == metav1.ConditionFalse {
		return metav1.ConditionFalse, conditions.ReasonSelfMonConfigNotGenerated
	}

	gatewayProbeResult, err := r.gatewayFlowHealthProber.Probe(ctx, pipeline.Name)
	if err != nil {
		logf.FromContext(ctx).Error(err, "Failed to probe flow health")
		return metav1.ConditionUnknown, conditions.ReasonSelfMonGatewayProbingFailed
	}

	logf.FromContext(ctx).V(1).Info("Probed gateway flow health", "result", gatewayProbeResult)

	// Probe agent flow health
	agentProbeResult, err := r.agentFlowHealthProber.Probe(ctx, pipeline.Name)
	if err != nil {
		logf.FromContext(ctx).Error(err, "Failed to probe agent flow health")
		return metav1.ConditionUnknown, conditions.ReasonSelfMonAgentProbingFailed
	}

	logf.FromContext(ctx).V(1).Info("Probed agent flow health", "result", agentProbeResult)

	reason := flowHealthReasonFor(gatewayProbeResult, agentProbeResult)
	if reason == conditions.ReasonSelfMonFlowHealthy {
		return metav1.ConditionTrue, reason
	}

	return metav1.ConditionFalse, reason
}

func flowHealthReasonFor(gatewayProbeResult prober.OTelGatewayProbeResult, agentProbeResult prober.OTelAgentProbeResult) string {
	switch {
	case gatewayProbeResult.AllDataDropped:
		return conditions.ReasonSelfMonGatewayAllDataDropped
	case gatewayProbeResult.SomeDataDropped:
		return conditions.ReasonSelfMonGatewaySomeDataDropped
	case gatewayProbeResult.Throttling:
		return conditions.ReasonSelfMonGatewayThrottling
	case agentProbeResult.AllDataDropped:
		return conditions.ReasonSelfMonAgentAllDataDropped
	case agentProbeResult.SomeDataDropped:
		return conditions.ReasonSelfMonAgentSomeDataDropped
	default:
		return conditions.ReasonSelfMonFlowHealthy
	}
}
