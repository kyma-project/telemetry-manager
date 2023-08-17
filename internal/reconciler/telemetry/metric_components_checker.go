package telemetry

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler"
)

type metricComponentsChecker struct {
	client client.Client
}

func (m *metricComponentsChecker) Check(ctx context.Context) (*metav1.Condition, error) {
	var metricPipelines v1alpha1.MetricPipelineList
	err := m.client.List(ctx, &metricPipelines)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to get all mertic pipelines while syncing conditions: %w", err)
	}

	reason := m.determineReason(metricPipelines.Items)
	return m.createConditionFromReason(reason), nil
}

func (m *metricComponentsChecker) determineReason(metricPipelines []v1alpha1.MetricPipeline) string {
	if len(metricPipelines) == 0 {
		return reconciler.ReasonNoPipelineDeployed
	}

	for _, pipeline := range metricPipelines {
		conditions := pipeline.Status.Conditions
		if len(conditions) == 0 {
			return reconciler.ReasonMetricGatewayDeploymentNotReady
		}

		lastCondition := conditions[len(conditions)-1]
		if lastCondition.Reason == reconciler.ReasonWaitingForLock {
			// Skip the case when user has deployed more than supported pipelines
			continue
		}
		if lastCondition.Reason == reconciler.ReasonReferencedSecretMissingReason {
			return reconciler.ReasonReferencedSecretMissingReason
		}
		if lastCondition.Type == v1alpha1.MetricPipelinePending {
			return reconciler.ReasonMetricGatewayDeploymentNotReady
		}
	}
	return reconciler.ReasonMetricGatewayDeploymentReady
}

func (m *metricComponentsChecker) createConditionFromReason(reason string) *metav1.Condition {
	if reason == reconciler.ReasonMetricGatewayDeploymentReady || reason == reconciler.ReasonNoPipelineDeployed {
		return &metav1.Condition{
			Type:    reconciler.MetricConditionType,
			Status:  reconciler.ConditionStatusTrue,
			Reason:  reason,
			Message: reconciler.Conditions[reason],
		}
	}
	return &metav1.Condition{
		Type:    reconciler.MetricConditionType,
		Status:  reconciler.ConditionStatusFalse,
		Reason:  reason,
		Message: reconciler.Conditions[reason],
	}
}
