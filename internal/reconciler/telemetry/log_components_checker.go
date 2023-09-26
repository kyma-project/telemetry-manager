package telemetry

import (
	"context"
	"fmt"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/extslices"
)

type logComponentsChecker struct {
	client client.Client
}

func (l *logComponentsChecker) Check(ctx context.Context, telemetryInDeletion bool) (*metav1.Condition, error) {
	var logPipelines v1alpha1.LogPipelineList
	err := l.client.List(ctx, &logPipelines)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to list log pipelines: %w", err)
	}

	var logParsers v1alpha1.LogParserList
	err = l.client.List(ctx, &logParsers)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to list log parsers: %w", err)
	}

	reason := l.determineReason(logPipelines.Items, logParsers.Items, telemetryInDeletion)
	message := l.createMessageForReason(logPipelines.Items, logParsers.Items, reason)
	return l.createConditionFromReason(reason, message), nil
}

func (l *logComponentsChecker) determineReason(pipelines []v1alpha1.LogPipeline, parsers []v1alpha1.LogParser, telemetryInDeletion bool) string {
	if telemetryInDeletion && (len(pipelines) != 0 || len(parsers) != 0) {
		return conditions.ReasonResourceBlocksDeletion
	}

	if len(pipelines) == 0 {
		return conditions.ReasonNoPipelineDeployed
	}

	if found := slices.ContainsFunc(pipelines, func(p v1alpha1.LogPipeline) bool {
		return l.isPendingWithReason(p, conditions.ReasonFluentBitDSNotReady)
	}); found {
		return conditions.ReasonFluentBitDSNotReady
	}

	if found := slices.ContainsFunc(pipelines, func(p v1alpha1.LogPipeline) bool {
		return l.isPendingWithReason(p, conditions.ReasonReferencedSecretMissing)
	}); found {
		return conditions.ReasonReferencedSecretMissing
	}

	return conditions.ReasonFluentBitDSReady
}

func (l *logComponentsChecker) isPendingWithReason(p v1alpha1.LogPipeline, reason string) bool {
	if len(p.Status.Conditions) == 0 {
		return false
	}

	lastCondition := p.Status.Conditions[len(p.Status.Conditions)-1]
	return lastCondition.Type == v1alpha1.LogPipelinePending && lastCondition.Reason == reason
}

func (l *logComponentsChecker) createMessageForReason(pipelines []v1alpha1.LogPipeline, parsers []v1alpha1.LogParser, reason string) string {
	if reason != conditions.ReasonResourceBlocksDeletion {
		return conditions.CommonMessageFor(reason)
	}

	return generateDeletionBlockedMessage(blockingResources{
		resourceType: "LogPipelines",
		resourceNames: extslices.TransformFunc(pipelines, func(p v1alpha1.LogPipeline) string {
			return p.Name
		}),
	}, blockingResources{
		resourceType: "LogParsers",
		resourceNames: extslices.TransformFunc(parsers, func(p v1alpha1.LogParser) string {
			return p.Name
		}),
	})
}

func (l *logComponentsChecker) createConditionFromReason(reason, message string) *metav1.Condition {
	conditionType := "LogComponentsHealthy"
	if reason == conditions.ReasonFluentBitDSReady || reason == conditions.ReasonNoPipelineDeployed {
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
