package metricpipeline

import (
	"context"
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *Reconciler) updateStatus(ctx context.Context, pipelineName string, lockAcquired bool) error {
	var pipeline telemetryv1alpha1.MetricPipeline
	if err := r.Get(ctx, types.NamespacedName{Name: pipelineName}, &pipeline); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("failed to get MetricPipeline: %w", err)
	}

	if pipeline.DeletionTimestamp != nil {
		return nil
	}

	if err := r.setAgentReadyCondition(ctx, &pipeline); err != nil {
		return fmt.Errorf("failed to set agent ready condition: %w", err)
	}

	if err := r.setGatewayReadyCondition(ctx, &pipeline); err != nil {
		return fmt.Errorf("failed to set gateway ready condition: %w", err)
	}

	if err := r.setGatewayConfigGeneratedCondition(ctx, &pipeline, lockAcquired); err != nil {
		return fmt.Errorf("failed to set gateway config generated condition: %w", err)
	}

	if err := r.Status().Update(ctx, &pipeline); err != nil {
		return fmt.Errorf("failed to update MetricPipeline status: %w", err)
	}

	return nil
}

func (r *Reconciler) setAgentReadyCondition(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline) error {
	status := metav1.ConditionTrue
	reason := conditions.ReasonMetricAgentNotRequired

	if isMetricAgentRequired(pipeline) {
		agentName := types.NamespacedName{Name: r.config.Agent.BaseName, Namespace: r.config.Agent.Namespace}
		ready, err := r.agentProber.IsReady(ctx, agentName)
		if err != nil {
			return fmt.Errorf("failed to probe agent %v: %w", agentName, err)
		}

		status = metav1.ConditionFalse
		reason = conditions.ReasonMetricAgentDaemonSetNotReady
		if ready {
			status = metav1.ConditionTrue
			reason = conditions.ReasonMetricAgentDaemonSetReady
		}
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, metav1.Condition{
		Type:    conditions.TypeMetricAgentHealthy,
		Status:  status,
		Reason:  reason,
		Message: conditions.CommonMessageFor(reason),
	})
	return nil
}

func (r *Reconciler) setGatewayReadyCondition(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline) error {
	gatewayName := types.NamespacedName{Name: r.config.Gateway.BaseName, Namespace: r.config.Gateway.Namespace}
	ready, err := r.gatewayProber.IsReady(ctx, gatewayName)
	if err != nil {
		return fmt.Errorf("failed to probe gateway %v: %w", gatewayName, err)
	}

	status := metav1.ConditionFalse
	reason := conditions.ReasonMetricGatewayDeploymentNotReady
	if ready {
		status = metav1.ConditionTrue
		reason = conditions.ReasonMetricGatewayDeploymentReady
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, metav1.Condition{
		Type:    conditions.TypeMetricGatewayHealthy,
		Status:  status,
		Reason:  reason,
		Message: conditions.CommonMessageFor(reason),
	})
	return nil
}

func (r *Reconciler) setGatewayConfigGeneratedCondition(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline, lockAcquired bool) error {
	status := metav1.ConditionTrue
	reason := conditions.ReasonMetricConfigurationGenerated
	if !lockAcquired {
		status = metav1.ConditionFalse
		reason = conditions.ReasonWaitingForLock
	}

	referencesNonExistentSecret := secretref.ReferencesNonExistentSecret(ctx, r.Client, pipeline)
	if referencesNonExistentSecret {
		status = metav1.ConditionFalse
		reason = conditions.ReasonReferencedSecretMissing
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, metav1.Condition{
		Type:    conditions.TypeConfigurationGenerated,
		Status:  status,
		Reason:  reason,
		Message: conditions.CommonMessageFor(reason),
	})
	return nil
}
