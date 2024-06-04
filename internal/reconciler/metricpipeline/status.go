package metricpipeline

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
)

func (r *Reconciler) updateStatus(ctx context.Context, pipelineName string, withinPipelineCountLimit bool) error {
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
	r.setGatewayConfigGeneratedCondition(ctx, &pipeline, withinPipelineCountLimit)
	r.setFlowHealthCondition(ctx, &pipeline)

	if err := r.Status().Update(ctx, &pipeline); err != nil {
		return fmt.Errorf("failed to update MetricPipeline status: %w", err)
	}

	return nil
}

func (r *Reconciler) setAgentHealthyCondition(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline) {
	status := metav1.ConditionTrue
	reason := conditions.ReasonMetricAgentNotRequired

	if isMetricAgentRequired(pipeline) {
		agentName := types.NamespacedName{Name: r.config.Agent.BaseName, Namespace: r.config.Agent.Namespace}
		healthy, err := r.agentProber.IsReady(ctx, agentName)
		if err != nil {
			logf.FromContext(ctx).V(1).Error(err, "Failed to probe metric agent - set condition as not healthy")
			healthy = false
		}

		status = metav1.ConditionFalse
		reason = conditions.ReasonAgentNotReady
		if healthy {
			status = metav1.ConditionTrue
			reason = conditions.ReasonAgentReady
		}
	}

	condition := metav1.Condition{
		Type:               conditions.TypeAgentHealthy,
		Status:             status,
		Reason:             reason,
		Message:            conditions.MessageForMetricPipeline(reason),
		ObservedGeneration: pipeline.Generation,
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, condition)
}

func (r *Reconciler) setGatewayHealthyCondition(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline) {
	gatewayName := types.NamespacedName{Name: r.config.Gateway.BaseName, Namespace: r.config.Gateway.Namespace}
	healthy, err := r.gatewayProber.IsReady(ctx, gatewayName)
	if err != nil {
		logf.FromContext(ctx).V(1).Error(err, "Failed to probe metric gateway - set condition as not healthy")
		healthy = false
	}

	status := metav1.ConditionFalse
	reason := conditions.ReasonGatewayNotReady
	if healthy {
		status = metav1.ConditionTrue
		reason = conditions.ReasonGatewayReady
	}

	condition := metav1.Condition{
		Type:               conditions.TypeGatewayHealthy,
		Status:             status,
		Reason:             reason,
		Message:            conditions.MessageForMetricPipeline(reason),
		ObservedGeneration: pipeline.Generation,
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, condition)
}

func (r *Reconciler) setGatewayConfigGeneratedCondition(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline, withinPipelineCountLimit bool) {

	status, reason, message := r.evaluateConfigGeneratedCondition(ctx, pipeline, withinPipelineCountLimit)
	condition := metav1.Condition{
		Type:               conditions.TypeConfigurationGenerated,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: pipeline.Generation,
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, condition)
}

func (r *Reconciler) evaluateConfigGeneratedCondition(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline, withinPipelineCountLimit bool) (status metav1.ConditionStatus, reason string, message string) {
	if !withinPipelineCountLimit {
		return metav1.ConditionFalse, conditions.ReasonMaxPipelinesExceeded, conditions.MessageForMetricPipeline(conditions.ReasonMaxPipelinesExceeded)
	}

	if secretref.ReferencesNonExistentSecret(ctx, r.Client, pipeline) {
		return metav1.ConditionFalse, conditions.ReasonReferencedSecretMissing, conditions.MessageForMetricPipeline(conditions.ReasonReferencedSecretMissing)
	}

	if tlsCertValidationRequired(pipeline) {
		cert := pipeline.Spec.Output.Otlp.TLS.Cert
		key := pipeline.Spec.Output.Otlp.TLS.Key

		err := r.tlsCertValidator.ValidateCertificate(ctx, cert, key)
		return conditions.EvaluateTLSCertCondition(err, conditions.ReasonGatewayConfigured, conditions.MessageForMetricPipeline(conditions.ReasonGatewayConfigured))
	}

	return metav1.ConditionTrue, conditions.ReasonGatewayConfigured, conditions.MessageForMetricPipeline(conditions.ReasonGatewayConfigured)
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
	if meta.IsStatusConditionFalse(pipeline.Status.Conditions, conditions.TypeConfigurationGenerated) {
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
