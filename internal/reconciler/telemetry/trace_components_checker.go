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

type traceComponentsChecker struct {
	client client.Client
}

func (t *traceComponentsChecker) Check(ctx context.Context, telemetryInDeletion bool) (*metav1.Condition, error) {
	var tracePipelines telemetryv1alpha1.TracePipelineList
	err := t.client.List(ctx, &tracePipelines)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to get list of TracePipelines: %w", err)
	}

	reason := t.determineReason(tracePipelines.Items, telemetryInDeletion)
	status := t.determineConditionStatus(reason)
	message := t.createMessageForReason(tracePipelines.Items, reason)
	reasonWithPrefix := t.addReasonPrefix(reason)

	conditionType := "TraceComponentsHealthy"
	return &metav1.Condition{
		Type:    conditionType,
		Status:  status,
		Reason:  reasonWithPrefix,
		Message: message,
	}, nil

}

func (t *traceComponentsChecker) determineReason(pipelines []telemetryv1alpha1.TracePipeline, telemetryInDeletion bool) string {
	if len(pipelines) == 0 {
		return conditions.ReasonNoPipelineDeployed
	}

	if telemetryInDeletion {
		return conditions.ReasonResourceBlocksDeletion
	}

	if reason := t.firstUnhealthyPipelineReason(pipelines); reason != "" {
		return reason
	}

	return conditions.ReasonTraceComponentsRunning
}

func (t *traceComponentsChecker) firstUnhealthyPipelineReason(pipelines []telemetryv1alpha1.TracePipeline) string {
	// condTypes order defines the priority of negative conditions
	condTypes := []string{
		conditions.TypeGatewayHealthy,
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

func (t *traceComponentsChecker) determineConditionStatus(reason string) metav1.ConditionStatus {
	if reason == conditions.ReasonNoPipelineDeployed || reason == conditions.ReasonTraceComponentsRunning {
		return metav1.ConditionTrue
	}
	return metav1.ConditionFalse
}

func (t *traceComponentsChecker) createMessageForReason(pipelines []telemetryv1alpha1.TracePipeline, reason string) string {
	if reason != conditions.ReasonResourceBlocksDeletion {
		return conditions.MessageFor(reason, conditions.TracesMessage)

	}

	return generateDeletionBlockedMessage(blockingResources{
		resourceType: "TracePipelines",
		resourceNames: extslices.TransformFunc(pipelines, func(p telemetryv1alpha1.TracePipeline) string {
			return p.Name
		}),
	})
}

func (t *traceComponentsChecker) addReasonPrefix(reason string) string {
	switch {
	case reason == conditions.ReasonDeploymentNotReady:
		return "TraceGateway" + reason
	case reason == conditions.ReasonReferencedSecretMissing:
		return "TracePipeline" + reason
	}
	return reason
}
