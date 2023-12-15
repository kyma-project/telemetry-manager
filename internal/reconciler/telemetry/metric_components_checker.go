package telemetry

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/extslices"
	"k8s.io/apimachinery/pkg/api/meta"
)

var metricPipelineReasonPrefixMap = map[string]string{
	conditions.TypeMetricGatewayHealthy:   "MetricGateway",
	conditions.TypeMetricAgentHealthy:     "MetricAgent",
	conditions.TypeConfigurationGenerated: "MetricPipelineConfiguration",
}

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
	message := m.createMessageForReason(metricPipelines.Items, reason)
	return m.createConditionFromReason(reason, message), nil
}

func (m *metricComponentsChecker) determineReason(pipelines []v1alpha1.MetricPipeline, telemetryInDeletion bool) string {
	if len(pipelines) == 0 {
		return conditions.ReasonNoPipelineDeployed
	}

	if telemetryInDeletion {
		return conditions.ReasonResourceBlocksDeletion
	}

	condTypes := []string{conditions.TypeMetricAgentHealthy, conditions.TypeMetricGatewayHealthy, conditions.TypeConfigurationGenerated}
	for _, pipeline := range pipelines {
		for _, condType := range condTypes {
			cond := meta.FindStatusCondition(pipeline.Status.Conditions, condType)
			if cond != nil && cond.Status == metav1.ConditionFalse {
				return metricPipelineReasonPrefixMap[cond.Type] + cond.Reason
			}
		}
	}

	return metricPipelineReasonPrefixMap[conditions.TypeMetricGatewayHealthy] + conditions.ReasonMetricGatewayDeploymentReady
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

func (m *metricComponentsChecker) createConditionFromReason(reason, message string) *metav1.Condition {
	conditionType := "MetricComponentsHealthy"
	if reason == conditions.ReasonMetricGatewayDeploymentReady || reason == conditions.ReasonNoPipelineDeployed {
		return &metav1.Condition{
			Type:    conditionType,
			Status:  metav1.ConditionTrue,
			Reason:  reason,
			Message: message,
		}
	}
	return &metav1.Condition{
		Type:    conditionType,
		Status:  metav1.ConditionFalse,
		Reason:  reason,
		Message: message,
	}
}
