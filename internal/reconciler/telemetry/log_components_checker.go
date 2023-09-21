package telemetry

import (
	"context"
	"fmt"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"strings"
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
		return conditions.ReasonLogResourceBlocksDeletion
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
	if reason != conditions.ReasonLogResourceBlocksDeletion {
		return conditions.CommonMessageFor(reason)
	}

	separator := ","
	var affectedResources []string
	if len(pipelines) > 0 {
		pipelineNames := extslices.TransformFunc(pipelines, func(p v1alpha1.LogPipeline) string {
			return p.Name
		})
		slices.Sort(pipelineNames)
		affectedResources = append(affectedResources, fmt.Sprintf("LogPipelines: (%s)", strings.Join(pipelineNames, separator)))
	}
	if len(parsers) > 0 {
		parserNames := extslices.TransformFunc(parsers, func(p v1alpha1.LogParser) string {
			return p.Name
		})
		slices.Sort(parserNames)
		affectedResources = append(affectedResources, fmt.Sprintf("LogParsers: (%s)", strings.Join(parserNames, separator)))
	}

	return fmt.Sprintf("The deletion of the module is blocked. To unblock the deletion, delete the following resources: %s",
		strings.Join(affectedResources, ", "))
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
