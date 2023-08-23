package telemetry

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler"
	"golang.org/x/exp/slices"
)

type metricComponentsChecker struct {
	client client.Client
}

func (m *metricComponentsChecker) Check(ctx context.Context) (*metav1.Condition, error) {
	var metricPipelines v1alpha1.MetricPipelineList
	err := m.client.List(ctx, &metricPipelines)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to get list metric pipelines: %w", err)
	}

	reason := m.determineReason(metricPipelines.Items)
	return m.createConditionFromReason(reason), nil
}

func (m *metricComponentsChecker) determineReason(pipelines []v1alpha1.MetricPipeline) string {
	if len(pipelines) == 0 {
		return reconciler.ReasonNoPipelineDeployed
	}

	if found := slices.ContainsFunc(pipelines, func(p v1alpha1.MetricPipeline) bool {
		return m.isPendingWithReason(p, reconciler.ReasonMetricGatewayDeploymentNotReady)
	}); found {
		return reconciler.ReasonMetricGatewayDeploymentNotReady
	}

	if found := slices.ContainsFunc(pipelines, func(p v1alpha1.MetricPipeline) bool {
		return m.isPendingWithReason(p, reconciler.ReasonReferencedSecretMissing)
	}); found {
		return reconciler.ReasonReferencedSecretMissing
	}

	return reconciler.ReasonMetricGatewayDeploymentReady
}

func (m *metricComponentsChecker) isPendingWithReason(p v1alpha1.MetricPipeline, reason string) bool {
	if len(p.Status.Conditions) == 0 {
		return false
	}

	lastCondition := p.Status.Conditions[len(p.Status.Conditions)-1]
	return lastCondition.Type == v1alpha1.MetricPipelinePending && lastCondition.Reason == reason
}

func (m *metricComponentsChecker) createConditionFromReason(reason string) *metav1.Condition {
	conditionType := "MetricComponentsHealthy"
	if reason == reconciler.ReasonMetricGatewayDeploymentReady || reason == reconciler.ReasonNoPipelineDeployed {
		return &metav1.Condition{
			Type:    conditionType,
			Status:  metav1.ConditionTrue,
			Reason:  reason,
			Message: reconciler.Condition(reason),
		}
	}
	return &metav1.Condition{
		Type:    conditionType,
		Status:  metav1.ConditionFalse,
		Reason:  reason,
		Message: reconciler.Condition(reason),
	}
}
