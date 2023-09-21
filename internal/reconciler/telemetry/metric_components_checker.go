package telemetry

import (
	"context"
	"fmt"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
)

type metricComponentsChecker struct {
	client client.Client
}

func (m *metricComponentsChecker) Check(ctx context.Context, telemetryInDeletion bool) (*metav1.Condition, error) {
	var metricPipelines v1alpha1.MetricPipelineList
	err := m.client.List(ctx, &metricPipelines)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to get list metric pipelines: %w", err)
	}

	reason := m.determineReason(metricPipelines.Items, telemetryInDeletion)
	return m.createConditionFromReason(reason), nil
}

func (m *metricComponentsChecker) determineReason(pipelines []v1alpha1.MetricPipeline, telemetryInDeletion bool) string {
	if len(pipelines) == 0 {
		return conditions.ReasonNoPipelineDeployed
	}

	if telemetryInDeletion {
		return conditions.ReasonMetricResourceBlocksDeletion
	}

	if found := slices.ContainsFunc(pipelines, func(p v1alpha1.MetricPipeline) bool {
		return m.isPendingWithReason(p, conditions.ReasonMetricGatewayDeploymentNotReady)
	}); found {
		return conditions.ReasonMetricGatewayDeploymentNotReady
	}

	if found := slices.ContainsFunc(pipelines, func(p v1alpha1.MetricPipeline) bool {
		return m.isPendingWithReason(p, conditions.ReasonReferencedSecretMissing)
	}); found {
		return conditions.ReasonReferencedSecretMissing
	}

	return conditions.ReasonMetricGatewayDeploymentReady
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
	if reason == conditions.ReasonMetricGatewayDeploymentReady || reason == conditions.ReasonNoPipelineDeployed {
		return &metav1.Condition{
			Type:    conditionType,
			Status:  metav1.ConditionTrue,
			Reason:  reason,
			Message: conditions.CommonMessageFor(reason),
		}
	}
	return &metav1.Condition{
		Type:    conditionType,
		Status:  metav1.ConditionFalse,
		Reason:  reason,
		Message: conditions.CommonMessageFor(reason),
	}
}
