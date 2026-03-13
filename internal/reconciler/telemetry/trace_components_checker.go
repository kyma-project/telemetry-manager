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

type traceComponentsChecker struct {
	client client.Client
}

func (t *traceComponentsChecker) Check(ctx context.Context, telemetryInDeletion bool) (*metav1.Condition, error) {
	var tracePipelines telemetryv1beta1.TracePipelineList

	err := t.client.List(ctx, &tracePipelines)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to get list of TracePipelines: %w", err)
	}

	if result := t.checkForResourceBlocksDeletionCondition(tracePipelines.Items, telemetryInDeletion); result != nil {
		return result, nil
	}

	if result := t.checkForNoPipelineDeployedCondition(tracePipelines.Items); result != nil {
		return result, nil
	}

	if result := t.checkForFirstUnhealthyPipelineCondition(tracePipelines.Items); result != nil {
		return result, nil
	}

	if result := t.checkForFirstAboutToExpirePipelineCondition(tracePipelines.Items); result != nil {
		return result, nil
	}

	return &metav1.Condition{
		Type:    conditions.TypeTraceComponentsHealthy,
		Status:  metav1.ConditionTrue,
		Reason:  conditions.ReasonComponentsRunning,
		Message: conditions.MessageForTracePipeline(conditions.ReasonComponentsRunning),
	}, nil
}

func (t *traceComponentsChecker) checkForFirstAboutToExpirePipelineCondition(pipelines []telemetryv1beta1.TracePipeline) *metav1.Condition {
	for _, pipeline := range pipelines {
		cond := meta.FindStatusCondition(pipeline.Status.Conditions, conditions.TypeConfigurationGenerated)
		if cond != nil && cond.Reason == conditions.ReasonTLSCertificateAboutToExpire {
			return &metav1.Condition{
				Type:    conditions.TypeTraceComponentsHealthy,
				Status:  cond.Status,
				Reason:  cond.Reason,
				Message: cond.Message,
			}
		}
	}

	return nil
}

func (t *traceComponentsChecker) checkForFirstUnhealthyPipelineCondition(pipelines []telemetryv1beta1.TracePipeline) *metav1.Condition {
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
					Type:    conditions.TypeTraceComponentsHealthy,
					Status:  cond.Status,
					Reason:  cond.Reason,
					Message: cond.Message,
				}
			}
		}
	}

	return nil
}

func (t *traceComponentsChecker) checkForNoPipelineDeployedCondition(pipelines []telemetryv1beta1.TracePipeline) *metav1.Condition {
	if len(pipelines) == 0 {
		return &metav1.Condition{
			Type:    conditions.TypeTraceComponentsHealthy,
			Status:  metav1.ConditionTrue,
			Reason:  conditions.ReasonNoPipelineDeployed,
			Message: conditions.MessageForTracePipeline(conditions.ReasonNoPipelineDeployed),
		}
	}

	return nil
}

func (t *traceComponentsChecker) checkForResourceBlocksDeletionCondition(pipelines []telemetryv1beta1.TracePipeline, telemetryInDeletion bool) *metav1.Condition {
	if telemetryInDeletion && len(pipelines) != 0 {
		return &metav1.Condition{
			Type:   conditions.TypeTraceComponentsHealthy,
			Status: metav1.ConditionFalse,
			Reason: conditions.ReasonResourceBlocksDeletion,
			Message: generateDeletionBlockedMessage(blockingResources{
				resourceType: "TracePipelines",
				resourceNames: slicesutils.TransformFunc(pipelines, func(p telemetryv1beta1.TracePipeline) string {
					return p.Name
				}),
			}),
		}
	}

	return nil
}
