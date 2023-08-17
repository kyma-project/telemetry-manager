package telemetry

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler"
)

type traceComponentsHealthChecker struct {
	client client.Client
}

func (t *traceComponentsHealthChecker) Check(ctx context.Context) (*metav1.Condition, error) {
	var tracePipelines v1alpha1.TracePipelineList
	err := t.client.List(ctx, &tracePipelines)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to get all trace pipelines while syncing conditions: %w", err)
	}
	if len(tracePipelines.Items) == 0 {
		return t.buildTelemetryConditions(reconciler.ReasonNoPipelineDeployed), nil
	}

	// Try to get the status of the trace collector via the pipelines
	status := t.validateTracePipeline(tracePipelines.Items)
	return t.buildTelemetryConditions(status), nil

}

func (t *traceComponentsHealthChecker) validateTracePipeline(tracePipeines []v1alpha1.TracePipeline) string {
	for _, m := range tracePipeines {
		conditions := m.Status.Conditions
		if len(conditions) == 0 {
			return reconciler.ReasonTraceCollectorDeploymentNotReady
		}
		if conditions[len(conditions)-1].Reason == reconciler.ReasonReferencedSecretMissingReason {
			return reconciler.ReasonReferencedSecretMissingReason
		}
		if conditions[len(conditions)-1].Type == v1alpha1.TracePipelinePending {
			return reconciler.ReasonTraceCollectorDeploymentNotReady
		}
	}
	return reconciler.ReasonTraceCollectorDeploymentReady
}

func (t *traceComponentsHealthChecker) buildTelemetryConditions(reason string) *metav1.Condition {
	if reason == reconciler.ReasonTraceCollectorDeploymentReady || reason == reconciler.ReasonNoPipelineDeployed {
		return &metav1.Condition{
			Type:    reconciler.TraceConditionType,
			Status:  reconciler.ConditionStatusTrue,
			Reason:  reason,
			Message: reconciler.Conditions[reason],
		}
	}
	return &metav1.Condition{
		Type:    reconciler.TraceConditionType,
		Status:  reconciler.ConditionStatusFalse,
		Reason:  reason,
		Message: reconciler.Conditions[reason],
	}
}
