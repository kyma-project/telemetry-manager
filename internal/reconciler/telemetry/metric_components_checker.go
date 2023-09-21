package telemetry

import (
	"context"
	"fmt"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/slicesext"
	"strings"
)

type metricComponentsChecker struct {
	client client.Client
}

func (m *metricComponentsChecker) Check(ctx context.Context, telemetryInDeletion bool) (*metav1.Condition, error) {
	var metricPipelines v1alpha1.MetricPipelineList
	err := m.client.List(ctx, &metricPipelines)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to get list metric pipelines: %w", err)
	}

	reason := m.determineReason(metricPipelines.Items, telemetryInDeletion)
	message := m.createMessageForReason(metricPipelines.Items, reason)
	return m.createConditionFromReason(reason, message), nil
}

func (m *metricComponentsChecker) determineReason(pipelines []v1alpha1.MetricPipeline, telemetryInDeletion bool) string {
	if len(pipelines) == 0 {
		return conditions.ReasonNoPipelineDeployed
	}

	if telemetryInDeletion {
		return conditions.ReasonMetricResourceBlocksDeletion
	}

	if found := slices.ContainsFunc(pipelines, func(p v1alpha1.MetricPipeline) bool {
		return m.isPendingWithReason(p, conditions.ReasonMetricGatewayDeploymentNotReady)
	}); found {
		return conditions.ReasonMetricGatewayDeploymentNotReady
	}

	if found := slices.ContainsFunc(pipelines, func(p v1alpha1.MetricPipeline) bool {
		return m.isPendingWithReason(p, conditions.ReasonReferencedSecretMissing)
	}); found {
		return conditions.ReasonReferencedSecretMissing
	}

	return conditions.ReasonMetricGatewayDeploymentReady
}

func (m *metricComponentsChecker) isPendingWithReason(p v1alpha1.MetricPipeline, reason string) bool {
	if len(p.Status.Conditions) == 0 {
		return false
	}

	lastCondition := p.Status.Conditions[len(p.Status.Conditions)-1]
	return lastCondition.Type == v1alpha1.MetricPipelinePending && lastCondition.Reason == reason
}

func (m *metricComponentsChecker) createMessageForReason(pipelines []v1alpha1.MetricPipeline, reason string) string {
	if reason != conditions.ReasonMetricResourceBlocksDeletion {
		return conditions.CommonMessageFor(reason)
	}

	pipelineNames := slicesext.TransformFunc(pipelines, func(p v1alpha1.MetricPipeline) string {
		return p.Name
	})
	slices.Sort(pipelineNames)
	separator := ","
	return fmt.Sprintf("The deletion of the module is blocked. To unblock the deletion, delete the following resources: MetricPipelines (%s)",
		strings.Join(pipelineNames, separator))
}

func (m *metricComponentsChecker) createConditionFromReason(reason, message string) *metav1.Condition {
	conditionType := "MetricComponentsHealthy"
	if reason == conditions.ReasonMetricGatewayDeploymentReady || reason == conditions.ReasonNoPipelineDeployed {
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
