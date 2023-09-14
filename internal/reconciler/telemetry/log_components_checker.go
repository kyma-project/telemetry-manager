package telemetry

import (
	"context"
	"fmt"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler"
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
	return l.createConditionFromReason(reason), nil
}

func (l *logComponentsChecker) determineReason(pipelines []v1alpha1.LogPipeline, parsers []v1alpha1.LogParser, telemetryInDeletion bool) string {
	if len(pipelines) == 0 {
		return reconciler.ReasonNoPipelineDeployed
	}

	if telemetryInDeletion && (len(pipelines) != 0 || len(parsers) != 0) {
		return reconciler.ReasonLogComponentsDeletionBlocked
	}

	if found := slices.ContainsFunc(pipelines, func(p v1alpha1.LogPipeline) bool {
		return l.isPendingWithReason(p, reconciler.ReasonFluentBitDSNotReady)
	}); found {
		return reconciler.ReasonFluentBitDSNotReady
	}

	if found := slices.ContainsFunc(pipelines, func(p v1alpha1.LogPipeline) bool {
		return l.isPendingWithReason(p, reconciler.ReasonReferencedSecretMissing)
	}); found {
		return reconciler.ReasonReferencedSecretMissing
	}

	return reconciler.ReasonFluentBitDSReady
}

func (l *logComponentsChecker) isPendingWithReason(p v1alpha1.LogPipeline, reason string) bool {
	if len(p.Status.Conditions) == 0 {
		return false
	}

	lastCondition := p.Status.Conditions[len(p.Status.Conditions)-1]
	return lastCondition.Type == v1alpha1.LogPipelinePending && lastCondition.Reason == reason
}

func (l *logComponentsChecker) createConditionFromReason(reason string) *metav1.Condition {
	conditionType := "LogComponentsHealthy"
	if reason == reconciler.ReasonFluentBitDSReady || reason == reconciler.ReasonNoPipelineDeployed {
		return &metav1.Condition{
			Type:    conditionType,
			Status:  metav1.ConditionTrue,
			Reason:  reason,
			Message: reconciler.Condition(reason),
		}
	}
	return &metav1.Condition{
		Type:    conditionType,
		Status:  metav1.ConditionFalse,
		Reason:  reason,
		Message: reconciler.Condition(reason),
	}
}
