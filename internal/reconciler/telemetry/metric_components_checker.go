package telemetry

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/extslices"
)

type metricComponentsChecker struct {
	client client.Client
}

func (m *metricComponentsChecker) Check(ctx context.Context, telemetryInDeletion bool) (*metav1.Condition, error) {
	var metricPipelines telemetryv1alpha1.MetricPipelineList
	err := m.client.List(ctx, &metricPipelines)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to get list of MetricPipelines: %w", err)
	}

	reason := m.determineReason(metricPipelines.Items, telemetryInDeletion)
	status := m.determineConditionStatus(reason)
	message := m.createMessageForReason(metricPipelines.Items, reason)
	reasonWithPrefix := m.addReasonPrefix(reason)

	const conditionType = "MetricComponentsHealthy"
	return &metav1.Condition{
		Type:    conditionType,
		Status:  status,
		Reason:  reasonWithPrefix,
		Message: message,
	}, nil
}

func (m *metricComponentsChecker) determineReason(pipelines []telemetryv1alpha1.MetricPipeline, telemetryInDeletion bool) string {
	if len(pipelines) == 0 {
		return conditions.ReasonNoPipelineDeployed
	}

	if telemetryInDeletion {
		return conditions.ReasonResourceBlocksDeletion
	}

	if reason := m.firstUnhealthyPipelineReason(pipelines); reason != "" {
		return reason
	}

	return conditions.ReasonMetricComponentsRunning
}

func (m *metricComponentsChecker) firstUnhealthyPipelineReason(pipelines []telemetryv1alpha1.MetricPipeline) string {
	// condTypes order defines the priority of negative conditions
	condTypes := []string{
		conditions.TypeGatewayHealthy,
		conditions.TypeAgentHealthy,
		conditions.TypeConfigurationGenerated,
	}
	for _, condType := range condTypes {
		for _, pipeline := range pipelines {
			cond := meta.FindStatusCondition(pipeline.Status.Conditions, condType)
			if cond != nil && cond.Status == metav1.ConditionFalse {
				return cond.Reason
			}
		}
	}
	return ""
}

func (m *metricComponentsChecker) determineConditionStatus(reason string) metav1.ConditionStatus {
	if reason == conditions.ReasonNoPipelineDeployed || reason == conditions.ReasonMetricComponentsRunning {
		return metav1.ConditionTrue
	}
	return metav1.ConditionFalse
}

func (m *metricComponentsChecker) createMessageForReason(pipelines []telemetryv1alpha1.MetricPipeline, reason string) string {
	if reason != conditions.ReasonResourceBlocksDeletion {
		return conditions.MessageFor(reason, conditions.MetricsMessage)
	}

	return generateDeletionBlockedMessage(blockingResources{
		resourceType: "MetricPipelines",
		resourceNames: extslices.TransformFunc(pipelines, func(p telemetryv1alpha1.MetricPipeline) string {
			return p.Name
		}),
	})
}

func (m *metricComponentsChecker) addReasonPrefix(reason string) string {
	switch {
	case reason == conditions.ReasonDeploymentNotReady:
		return "MetricGateway" + reason
	case reason == conditions.ReasonDaemonSetNotReady:
		return "MetricAgent" + reason
	case reason == conditions.ReasonReferencedSecretMissing:
		return "MetricPipeline" + reason
	}
	return reason
}
