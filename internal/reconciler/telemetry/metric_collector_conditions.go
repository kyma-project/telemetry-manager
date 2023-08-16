package telemetry

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler"
)

type metricComponentsHealthChecker struct {
	client client.Client
}

func (m *metricComponentsHealthChecker) check(ctx context.Context) (*metav1.Condition, error) {
	var metricPipelines v1alpha1.MetricPipelineList
	err := m.client.List(ctx, &metricPipelines)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to get all mertic pipelines while syncing conditions: %w", err)
	}

	if len(metricPipelines.Items) == 0 {
		return m.buildTelemetryConditions(reconciler.ReasonNoPipelineDeployed), nil
	}

	// Try to get the status of the metric collector via the pipelines
	status := m.validateMetricPipeline(metricPipelines.Items)
	return m.buildTelemetryConditions(status), nil
}

func (m *metricComponentsHealthChecker) validateMetricPipeline(metricPipelines []v1alpha1.MetricPipeline) string {
	for _, m := range metricPipelines {
		conditions := m.Status.Conditions
		if len(conditions) == 0 {
			return reconciler.ReasonMetricGatewayDeploymentNotReady
		}
		if conditions[len(conditions)-1].Reason == reconciler.ReasonReferencedSecretMissingReason {
			return reconciler.ReasonReferencedSecretMissingReason
		}
		if conditions[len(conditions)-1].Type == v1alpha1.MetricPipelinePending {
			return reconciler.ReasonMetricGatewayDeploymentNotReady
		}
	}
	return reconciler.ReasonMetricGatewayDeploymentReady
}

func (m *metricComponentsHealthChecker) buildTelemetryConditions(reason string) *metav1.Condition {
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
