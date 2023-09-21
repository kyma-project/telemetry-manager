package telemetry

import (
	"context"
	"fmt"
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/extslices"
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

	reason := t.determineReason(tracePipelines.Items, telemetryInDeletion)
	message := t.createMessageForReason(tracePipelines.Items, reason)
	return t.createConditionFromReason(reason, message), nil

}

func (t *traceComponentsChecker) determineReason(pipelines []v1alpha1.TracePipeline, telemetryInDeletion bool) string {
	if len(pipelines) == 0 {
		return conditions.ReasonNoPipelineDeployed
	}

	if telemetryInDeletion {
		return conditions.ReasonResourceBlocksDeletion
	}
	if found := slices.ContainsFunc(pipelines, func(p v1alpha1.TracePipeline) bool {
		return t.isPendingWithReason(p, conditions.ReasonTraceGatewayDeploymentNotReady)
	}); found {
		return conditions.ReasonTraceGatewayDeploymentNotReady
	}

	if found := slices.ContainsFunc(pipelines, func(p v1alpha1.TracePipeline) bool {
		return t.isPendingWithReason(p, conditions.ReasonReferencedSecretMissing)
	}); found {
		return conditions.ReasonReferencedSecretMissing
	}

	return conditions.ReasonTraceGatewayDeploymentReady
}

func (t *traceComponentsChecker) isPendingWithReason(p v1alpha1.TracePipeline, reason string) bool {
	if len(p.Status.Conditions) == 0 {
		return false
	}

	lastCondition := p.Status.Conditions[len(p.Status.Conditions)-1]
	return lastCondition.Type == v1alpha1.TracePipelinePending && lastCondition.Reason == reason
}

func (t *traceComponentsChecker) createMessageForReason(pipelines []v1alpha1.TracePipeline, reason string) string {
	if reason != conditions.ReasonResourceBlocksDeletion {
		return conditions.CommonMessageFor(reason)

	}

	pipelineNames := extslices.TransformFunc(pipelines, func(p v1alpha1.TracePipeline) string {
		return p.Name
	})
	slices.Sort(pipelineNames)
	separator := ","
	return fmt.Sprintf("The deletion of the module is blocked. To unblock the deletion, delete the following resources: TracePipelines (%s)",
		strings.Join(pipelineNames, separator))
}

func (t *traceComponentsChecker) createConditionFromReason(reason, message string) *metav1.Condition {
	conditionType := "TraceComponentsHealthy"
	if reason == conditions.ReasonTraceGatewayDeploymentReady || reason == conditions.ReasonNoPipelineDeployed {
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
