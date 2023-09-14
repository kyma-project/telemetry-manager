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

type traceComponentsChecker struct {
	client client.Client
}

func (t *traceComponentsChecker) Check(ctx context.Context, telemetryInDeletion bool) (*metav1.Condition, error) {
	var tracePipelines v1alpha1.TracePipelineList
	err := t.client.List(ctx, &tracePipelines)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to get all trace pipelines while syncing conditions: %w", err)
	}

	status := t.determineReason(tracePipelines.Items, telemetryInDeletion)
	return t.createConditionFromReason(status), nil

}

func (t *traceComponentsChecker) determineReason(pipelines []v1alpha1.TracePipeline, telemetryInDeletion bool) string {
	if len(pipelines) == 0 {
		return reconciler.ReasonNoPipelineDeployed
	}

	if telemetryInDeletion {
		return reconciler.ReasonTraceResourceBlocksDeletion
	}
	if found := slices.ContainsFunc(pipelines, func(p v1alpha1.TracePipeline) bool {
		return t.isPendingWithReason(p, reconciler.ReasonTraceGatewayDeploymentNotReady)
	}); found {
		return reconciler.ReasonTraceGatewayDeploymentNotReady
	}

	if found := slices.ContainsFunc(pipelines, func(p v1alpha1.TracePipeline) bool {
		return t.isPendingWithReason(p, reconciler.ReasonReferencedSecretMissing)
	}); found {
		return reconciler.ReasonReferencedSecretMissing
	}

	return reconciler.ReasonTraceGatewayDeploymentReady
}

func (t *traceComponentsChecker) isPendingWithReason(p v1alpha1.TracePipeline, reason string) bool {
	if len(p.Status.Conditions) == 0 {
		return false
	}

	lastCondition := p.Status.Conditions[len(p.Status.Conditions)-1]
	return lastCondition.Type == v1alpha1.TracePipelinePending && lastCondition.Reason == reason
}

func (t *traceComponentsChecker) createConditionFromReason(reason string) *metav1.Condition {
	conditionType := "TraceComponentsHealthy"
	if reason == reconciler.ReasonTraceGatewayDeploymentReady || reason == reconciler.ReasonNoPipelineDeployed {
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
