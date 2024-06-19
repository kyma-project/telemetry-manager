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

type logComponentsChecker struct {
	client client.Client
}

func (l *logComponentsChecker) Check(ctx context.Context, telemetryInDeletion bool) (*metav1.Condition, error) {
	var logPipelines telemetryv1alpha1.LogPipelineList
	err := l.client.List(ctx, &logPipelines)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to get list of LogPipelines: %w", err)
	}

	var logParsers telemetryv1alpha1.LogParserList
	err = l.client.List(ctx, &logParsers)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to get list of LogParsers: %w", err)
	}

	if result := l.checkForResourseBlocksDeletionCondition(logPipelines.Items, logParsers.Items, telemetryInDeletion); result != nil {
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
		Message: conditions.MessageForLogPipeline(conditions.ReasonComponentsRunning),
	}, nil
}

func (l *logComponentsChecker) checkForFirstAboutToExpirePipelineCondition(pipelines []telemetryv1alpha1.LogPipeline) *metav1.Condition {
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

func (l *logComponentsChecker) checkForFirstUnhealthyPipelineCondition(pipelines []telemetryv1alpha1.LogPipeline) *metav1.Condition {
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

func (l *logComponentsChecker) checkForNoPipelineDeployedCondition(pipelines []telemetryv1alpha1.LogPipeline) *metav1.Condition {
	if len(pipelines) == 0 {
		return &metav1.Condition{
			Type:    conditions.TypeLogComponentsHealthy,
			Status:  metav1.ConditionTrue,
			Reason:  conditions.ReasonNoPipelineDeployed,
			Message: conditions.MessageForLogPipeline(conditions.ReasonNoPipelineDeployed),
		}
	}
	return nil
}

func (l *logComponentsChecker) checkForResourseBlocksDeletionCondition(pipelines []telemetryv1alpha1.LogPipeline, parsers []telemetryv1alpha1.LogParser, telemetryInDeletion bool) *metav1.Condition {
	if telemetryInDeletion && (len(pipelines) != 0 || len(parsers) != 0) {
		return &metav1.Condition{
			Type:   conditions.TypeLogComponentsHealthy,
			Status: metav1.ConditionFalse,
			Reason: conditions.ReasonResourceBlocksDeletion,
			Message: generateDeletionBlockedMessage(blockingResources{
				resourceType: "LogPipelines",
				resourceNames: extslices.TransformFunc(pipelines, func(p telemetryv1alpha1.LogPipeline) string {
					return p.Name
				}),
			}, blockingResources{
				resourceType: "LogParsers",
				resourceNames: extslices.TransformFunc(parsers, func(p telemetryv1alpha1.LogParser) string {
					return p.Name
				}),
			}),
		}
	}
	return nil
}
