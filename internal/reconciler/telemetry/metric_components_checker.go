package telemetry

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	slicesutils "github.com/kyma-project/telemetry-manager/internal/utils/slices"
)

type metricComponentsChecker struct {
	client client.Client
}

func (m *metricComponentsChecker) Check(ctx context.Context, telemetryInDeletion bool) (*metav1.Condition, error) {
	var metricPipelines telemetryv1beta1.MetricPipelineList

	err := m.client.List(ctx, &metricPipelines)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to get list of MetricPipelines: %w", err)
	}

	if result := m.checkForResourceBlocksDeletionCondition(metricPipelines.Items, telemetryInDeletion); result != nil {
		return result, nil
	}

	if result := m.checkForNoPipelineDeployedCondition(metricPipelines.Items); result != nil {
		return result, nil
	}

	if result := m.checkForFirstUnhealthyPipelineCondition(metricPipelines.Items); result != nil {
		return result, nil
	}

	if result := m.checkForFirstAboutToExpirePipelineCondition(metricPipelines.Items); result != nil {
		return result, nil
	}

	return &metav1.Condition{
		Type:    conditions.TypeMetricComponentsHealthy,
		Status:  metav1.ConditionTrue,
		Reason:  conditions.ReasonComponentsRunning,
		Message: conditions.MessageForMetricPipeline(conditions.ReasonComponentsRunning),
	}, nil
}

func (m *metricComponentsChecker) checkForFirstAboutToExpirePipelineCondition(pipelines []telemetryv1beta1.MetricPipeline) *metav1.Condition {
	for _, pipeline := range pipelines {
		cond := meta.FindStatusCondition(pipeline.Status.Conditions, conditions.TypeConfigurationGenerated)
		if cond != nil && cond.Reason == conditions.ReasonTLSCertificateAboutToExpire {
			return &metav1.Condition{
				Type:    conditions.TypeMetricComponentsHealthy,
				Status:  cond.Status,
				Reason:  cond.Reason,
				Message: cond.Message,
			}
		}
	}

	return nil
}

func (m *metricComponentsChecker) checkForFirstUnhealthyPipelineCondition(pipelines []telemetryv1beta1.MetricPipeline) *metav1.Condition {
	// condTypes order defines the priority of negative conditions
	condTypes := []string{
		conditions.TypeConfigurationGenerated,
		conditions.TypeGatewayHealthy,
		conditions.TypeAgentHealthy,
		conditions.TypeFlowHealthy,
	}

	for _, condType := range condTypes {
		for _, pipeline := range pipelines {
			cond := meta.FindStatusCondition(pipeline.Status.Conditions, condType)
			if cond != nil && cond.Status == metav1.ConditionFalse {
				return &metav1.Condition{
					Type:    conditions.TypeMetricComponentsHealthy,
					Status:  cond.Status,
					Reason:  cond.Reason,
					Message: cond.Message,
				}
			}
		}
	}

	return nil
}

func (m *metricComponentsChecker) checkForNoPipelineDeployedCondition(pipelines []telemetryv1beta1.MetricPipeline) *metav1.Condition {
	if len(pipelines) == 0 {
		return &metav1.Condition{
			Type:    conditions.TypeMetricComponentsHealthy,
			Status:  metav1.ConditionTrue,
			Reason:  conditions.ReasonNoPipelineDeployed,
			Message: conditions.MessageForMetricPipeline(conditions.ReasonNoPipelineDeployed),
		}
	}

	return nil
}

func (m *metricComponentsChecker) checkForResourceBlocksDeletionCondition(pipelines []telemetryv1beta1.MetricPipeline, telemetryInDeletion bool) *metav1.Condition {
	if telemetryInDeletion && len(pipelines) != 0 {
		return &metav1.Condition{
			Type:   conditions.TypeMetricComponentsHealthy,
			Status: metav1.ConditionFalse,
			Reason: conditions.ReasonResourceBlocksDeletion,
			Message: generateDeletionBlockedMessage(blockingResources{
				resourceType: "MetricPipelines",
				resourceNames: slicesutils.TransformFunc(pipelines, func(p telemetryv1beta1.MetricPipeline) string {
					return p.Name
				}),
			}),
		}
	}

	return nil
}
