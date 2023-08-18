package telemetry

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler"
)

type traceComponentsChecker struct {
	client client.Client
}

func (t *traceComponentsChecker) Check(ctx context.Context) (*metav1.Condition, error) {
	var tracePipelines v1alpha1.TracePipelineList
	err := t.client.List(ctx, &tracePipelines)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to get all trace pipelines while syncing conditions: %w", err)
	}
	if len(tracePipelines.Items) == 0 {
		return t.createConditionFromReason(reconciler.ReasonNoPipelineDeployed), nil
	}

	// Try to get the status of the trace collector via the pipelines
	status := t.determineReason(tracePipelines.Items)
	return t.createConditionFromReason(status), nil

}

func (t *traceComponentsChecker) determineReason(tracePipelines []v1alpha1.TracePipeline) string {
	if len(tracePipelines) == 0 {
		return reconciler.ReasonNoPipelineDeployed
	}

	for _, pipeline := range tracePipelines {
		conditions := pipeline.Status.Conditions
		if len(conditions) == 0 {
			return reconciler.ReasonTraceGatewayDeploymentNotReady
		}

		lastCondition := conditions[len(conditions)-1]
		if lastCondition.Reason == reconciler.ReasonWaitingForLock {
			// Skip the case when user has deployed more than supported pipelines
			continue
		}
		if lastCondition.Reason == reconciler.ReasonReferencedSecretMissing {
			return reconciler.ReasonReferencedSecretMissing
		}
		if lastCondition.Type == v1alpha1.TracePipelinePending {
			return reconciler.ReasonTraceGatewayDeploymentNotReady
		}
	}
	return reconciler.ReasonTraceGatewayDeploymentReady
}

func (t *traceComponentsChecker) createConditionFromReason(reason string) *metav1.Condition {
	if reason == reconciler.ReasonTraceGatewayDeploymentReady || reason == reconciler.ReasonNoPipelineDeployed {
		return &metav1.Condition{
			Type:    traceComponentsHealthyConditionType,
			Status:  metav1.ConditionTrue,
			Reason:  reason,
			Message: reconciler.Condition(reason),
		}
	}
	return &metav1.Condition{
		Type:    traceComponentsHealthyConditionType,
		Status:  metav1.ConditionFalse,
		Reason:  reason,
		Message: reconciler.Condition(reason),
	}
}
