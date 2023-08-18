package telemetry

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler"
)

type logComponentsChecker struct {
	client client.Client
}

func (l *logComponentsChecker) Check(ctx context.Context) (*metav1.Condition, error) {
	var logPipelines v1alpha1.LogPipelineList
	err := l.client.List(ctx, &logPipelines)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to list log pipelines: %w", err)
	}

	reason := l.determineReason(logPipelines.Items)
	return l.createConditionFromReason(reason), nil
}

func (l *logComponentsChecker) determineReason(logPipelines []v1alpha1.LogPipeline) string {
	if len(logPipelines) == 0 {
		return reconciler.ReasonNoPipelineDeployed
	}

	for _, pipeline := range logPipelines {
		conditions := pipeline.Status.Conditions
		if len(conditions) == 0 {
			return reconciler.ReasonFluentBitDSNotReady
		}
		lastCondition := conditions[len(conditions)-1]
		if lastCondition.Reason == reconciler.ReasonReferencedSecretMissing {
			return reconciler.ReasonReferencedSecretMissing
		}
		if lastCondition.Type == v1alpha1.LogPipelinePending {
			return reconciler.ReasonFluentBitDSNotReady
		}
	}
	return reconciler.ReasonFluentBitDSReady
}

func (l *logComponentsChecker) createConditionFromReason(reason string) *metav1.Condition {
	if reason == reconciler.ReasonFluentBitDSReady || reason == reconciler.ReasonNoPipelineDeployed {
		return &metav1.Condition{
			Type:    logComponentsHealthyConditionType,
			Status:  metav1.ConditionTrue,
			Reason:  reason,
			Message: reconciler.Condition(reason),
		}
	}
	return &metav1.Condition{
		Type:    logComponentsHealthyConditionType,
		Status:  metav1.ConditionFalse,
		Reason:  reason,
		Message: reconciler.Condition(reason),
	}
}
