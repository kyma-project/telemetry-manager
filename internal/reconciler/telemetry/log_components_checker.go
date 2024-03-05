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

	reason := l.determineReason(logPipelines.Items, logParsers.Items, telemetryInDeletion)
	status := l.determineConditionStatus(reason)
	message := l.createMessageForReason(logPipelines.Items, logParsers.Items, reason)
	reasonWithPrefix := l.addReasonPrefix(reason)

	conditionType := "LogComponentsHealthy"
	return &metav1.Condition{
		Type:    conditionType,
		Status:  status,
		Reason:  reasonWithPrefix,
		Message: message,
	}, nil
}

func (l *logComponentsChecker) determineReason(pipelines []telemetryv1alpha1.LogPipeline, parsers []telemetryv1alpha1.LogParser, telemetryInDeletion bool) string {
	if telemetryInDeletion && (len(pipelines) != 0 || len(parsers) != 0) {
		return conditions.ReasonResourceBlocksDeletion
	}

	if len(pipelines) == 0 {
		return conditions.ReasonNoPipelineDeployed
	}

	if reason := l.firstUnhealthyPipelineReason(pipelines); reason != "" {
		return reason
	}

	return conditions.ReasonLogComponentsRunning
}

func (l *logComponentsChecker) firstUnhealthyPipelineReason(pipelines []telemetryv1alpha1.LogPipeline) string {
	// condTypes order defines the priority of negative conditions
	condTypes := []string{
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

func (l *logComponentsChecker) determineConditionStatus(reason string) metav1.ConditionStatus {
	if reason == conditions.ReasonNoPipelineDeployed || reason == conditions.ReasonLogComponentsRunning {
		return metav1.ConditionTrue
	}
	return metav1.ConditionFalse
}

func (l *logComponentsChecker) createMessageForReason(pipelines []telemetryv1alpha1.LogPipeline, parsers []telemetryv1alpha1.LogParser, reason string) string {
	if reason != conditions.ReasonResourceBlocksDeletion {
		return conditions.MessageFor(reason, conditions.LogsMessage)
	}

	return generateDeletionBlockedMessage(blockingResources{
		resourceType: "LogPipelines",
		resourceNames: extslices.TransformFunc(pipelines, func(p telemetryv1alpha1.LogPipeline) string {
			return p.Name
		}),
	}, blockingResources{
		resourceType: "LogParsers",
		resourceNames: extslices.TransformFunc(parsers, func(p telemetryv1alpha1.LogParser) string {
			return p.Name
		}),
	})
}

func (l *logComponentsChecker) addReasonPrefix(reason string) string {
	switch {
	case reason == conditions.ReasonDaemonSetNotReady:
		return "FluentBit" + reason
	case reason == conditions.ReasonReferencedSecretMissing:
		return "LogPipeline" + reason
	}
	return reason
}
