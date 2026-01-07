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

type logComponentsChecker struct {
	client client.Client
}

func (l *logComponentsChecker) Check(ctx context.Context, telemetryInDeletion bool) (*metav1.Condition, error) {
	var logPipelines telemetryv1beta1.LogPipelineList

	err := l.client.List(ctx, &logPipelines)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to get list of LogPipelines: %w", err)
	}

	if result := l.checkForResourceBlocksDeletionCondition(logPipelines.Items, telemetryInDeletion); result != nil {
		return result, nil
	}

	if result := l.checkForNoPipelineDeployedCondition(logPipelines.Items); result != nil {
		return result, nil
	}

	if result := l.checkForFirstUnhealthyPipelineCondition(logPipelines.Items); result != nil {
		return result, nil
	}

	if result := l.checkForFirstAboutToExpirePipelineCondition(logPipelines.Items); result != nil {
		return result, nil
	}

	return &metav1.Condition{
		Type:    conditions.TypeLogComponentsHealthy,
		Status:  metav1.ConditionTrue,
		Reason:  conditions.ReasonComponentsRunning,
		Message: conditions.MessageForFluentBitLogPipeline(conditions.ReasonComponentsRunning),
	}, nil
}

func (l *logComponentsChecker) checkForFirstAboutToExpirePipelineCondition(pipelines []telemetryv1beta1.LogPipeline) *metav1.Condition {
	for _, pipeline := range pipelines {
		cond := meta.FindStatusCondition(pipeline.Status.Conditions, conditions.TypeConfigurationGenerated)
		if cond != nil && cond.Reason == conditions.ReasonTLSCertificateAboutToExpire {
			return &metav1.Condition{
				Type:    conditions.TypeLogComponentsHealthy,
				Status:  cond.Status,
				Reason:  cond.Reason,
				Message: cond.Message,
			}
		}
	}

	return nil
}

func (l *logComponentsChecker) checkForFirstUnhealthyPipelineCondition(pipelines []telemetryv1beta1.LogPipeline) *metav1.Condition {
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
					Type:    conditions.TypeLogComponentsHealthy,
					Status:  cond.Status,
					Reason:  cond.Reason,
					Message: cond.Message,
				}
			}
		}
	}

	return nil
}

func (l *logComponentsChecker) checkForNoPipelineDeployedCondition(pipelines []telemetryv1beta1.LogPipeline) *metav1.Condition {
	if len(pipelines) == 0 {
		return &metav1.Condition{
			Type:    conditions.TypeLogComponentsHealthy,
			Status:  metav1.ConditionTrue,
			Reason:  conditions.ReasonNoPipelineDeployed,
			Message: conditions.MessageForFluentBitLogPipeline(conditions.ReasonNoPipelineDeployed),
		}
	}

	return nil
}

func (l *logComponentsChecker) checkForResourceBlocksDeletionCondition(pipelines []telemetryv1beta1.LogPipeline, telemetryInDeletion bool) *metav1.Condition {
	if telemetryInDeletion && (len(pipelines) != 0) {
		return &metav1.Condition{
			Type:   conditions.TypeLogComponentsHealthy,
			Status: metav1.ConditionFalse,
			Reason: conditions.ReasonResourceBlocksDeletion,
			Message: generateDeletionBlockedMessage(blockingResources{
				resourceType: "LogPipelines",
				resourceNames: slicesutils.TransformFunc(pipelines, func(p telemetryv1beta1.LogPipeline) string {
					return p.Name
				}),
			}),
		}
	}

	return nil
}
