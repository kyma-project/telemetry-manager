package metricpipeline

import (
	"context"
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/prometheus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
)

func (r *Reconciler) updateStatus(ctx context.Context, pipelineName string, withinPipelineCountLimit bool, alert prometheus.Alerts) error {
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
	r.setMetricFlowHealthCondition(&pipeline, alert)

	if err := r.Status().Update(ctx, &pipeline); err != nil {
		return fmt.Errorf("failed to update MetricPipeline status: %w", err)
	}
	fmt.Printf("Fetching conditions after update: %v\n", &pipeline.Status.Conditions)

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
		reason = conditions.ReasonMetricAgentDaemonSetNotReady
		if healthy {
			status = metav1.ConditionTrue
			reason = conditions.ReasonMetricAgentDaemonSetReady
		}
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, newCondition(conditions.TypeMetricAgentHealthy, reason, status, pipeline.Generation))
}

func (r *Reconciler) setGatewayHealthyCondition(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline) {
	gatewayName := types.NamespacedName{Name: r.config.Gateway.BaseName, Namespace: r.config.Gateway.Namespace}
	healthy, err := r.gatewayProber.IsReady(ctx, gatewayName)
	if err != nil {
		logf.FromContext(ctx).V(1).Error(err, "Failed to probe metric gateway - set condition as not healthy")
		healthy = false
	}

	status := metav1.ConditionFalse
	reason := conditions.ReasonMetricGatewayDeploymentNotReady
	if healthy {
		status = metav1.ConditionTrue
		reason = conditions.ReasonMetricGatewayDeploymentReady
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, newCondition(conditions.TypeMetricGatewayHealthy, reason, status, pipeline.Generation))
}

func (r *Reconciler) setGatewayConfigGeneratedCondition(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline, withinPipelineCountLimit bool) {
	status := metav1.ConditionTrue
	reason := conditions.ReasonMetricConfigurationGenerated

	if secretref.ReferencesNonExistentSecret(ctx, r.Client, pipeline) {
		status = metav1.ConditionFalse
		reason = conditions.ReasonReferencedSecretMissing
	}

	if !withinPipelineCountLimit {
		status = metav1.ConditionFalse
		reason = conditions.ReasonMaxPipelinesExceeded
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, newCondition(conditions.TypeConfigurationGenerated, reason, status, pipeline.Generation))
}

func (r *Reconciler) setMetricFlowHealthCondition(pipeline *telemetryv1alpha1.MetricPipeline, alert prometheus.Alerts) {
	fmt.Printf("Alertname is: %v\n", alert.Name)
	status := metav1.ConditionTrue
	reason := conditions.ReasonMetricFlowHealthy

	if alert.Name != "" {
		status = metav1.ConditionFalse
		reason = conditions.FetchReasonFromAlert(alert)
	}
	msg := conditions.MessageForAlerts(alert)

	fmt.Printf("Status is: %v, Reason: %v, msg: %v\n", status, reason)

	meta.SetStatusCondition(&pipeline.Status.Conditions, newConditionForAlerts(conditions.TypeMetricFlowHealthy, reason, msg, status, pipeline.Generation))
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

func newConditionForAlerts(condType, reason, msg string, status metav1.ConditionStatus, generation int64) metav1.Condition {
	return metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            msg,
		ObservedGeneration: generation,
	}
}
