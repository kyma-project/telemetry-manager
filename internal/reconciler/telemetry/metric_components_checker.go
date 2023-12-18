package telemetry

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/extslices"
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

func (m *metricComponentsChecker) determineReason(pipelines []v1alpha1.MetricPipeline, telemetryInDeletion bool) string {
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

func (m *metricComponentsChecker) firstUnhealthyPipelineReason(pipelines []v1alpha1.MetricPipeline) string {
	// condTypes order defines the priority of negative conditions
	condTypes := []string{
		conditions.TypeMetricGatewayHealthy,
		conditions.TypeMetricAgentHealthy,
		conditions.TypeConfigurationGenerated,
	}
	for _, pipeline := range pipelines {
		for _, condType := range condTypes {
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

func (m *metricComponentsChecker) createMessageForReason(pipelines []v1alpha1.MetricPipeline, reason string) string {
	if reason != conditions.ReasonResourceBlocksDeletion {
		return conditions.CommonMessageFor(reason)
	}

	return generateDeletionBlockedMessage(blockingResources{
		resourceType: "MetricPipelines",
		resourceNames: extslices.TransformFunc(pipelines, func(p v1alpha1.MetricPipeline) string {
			return p.Name
		}),
	})
}

func (m *metricComponentsChecker) addReasonPrefix(reason string) string {
	switch reason {
	case conditions.ReasonMetricGatewayDeploymentReady:
	case conditions.ReasonMetricGatewayDeploymentNotReady:
		return "MetricGateway" + reason
	case conditions.ReasonMetricAgentDaemonSetReady:
	case conditions.ReasonMetricAgentDaemonSetNotReady:
		return "MetricAgent" + reason

	case conditions.ReasonWaitingForLock:
	case conditions.ReasonReferencedSecretMissing:
		return "MetricPipeline" + reason
	}
	return reason
}
