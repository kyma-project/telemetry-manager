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

type metricComponentsChecker struct {
	client                   client.Client
	flowHealthProbingEnabled bool
}

func (m *metricComponentsChecker) Check(ctx context.Context, telemetryInDeletion bool) (*metav1.Condition, error) {
	var metricPipelines telemetryv1alpha1.MetricPipelineList
	err := m.client.List(ctx, &metricPipelines)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to get list of MetricPipelines: %w", err)
	}

	reason := m.determineReason(metricPipelines.Items, telemetryInDeletion)
	status := m.determineConditionStatus(reason)
	message := m.createMessageForReason(metricPipelines.Items, reason)

	conditionType := conditions.TypeMetricComponentsHealthy
	return &metav1.Condition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	}, nil
}

func (m *metricComponentsChecker) determineReason(pipelines []telemetryv1alpha1.MetricPipeline, telemetryInDeletion bool) string {
	if len(pipelines) == 0 {
		return conditions.ReasonNoPipelineDeployed
	}

	if telemetryInDeletion {
		return conditions.ReasonResourceBlocksDeletion
	}

	if reason := m.firstUnhealthyPipelineReason(pipelines); reason != "" {
		return reason
	}

	for _, pipeline := range pipelines {
		cond := meta.FindStatusCondition(pipeline.Status.Conditions, conditions.TypeConfigurationGenerated)
		if cond != nil && cond.Reason == conditions.ReasonTLSCertificateAboutToExpire {
			return cond.Reason
		}
	}

	return conditions.ReasonComponentsRunning
}

func (m *metricComponentsChecker) firstUnhealthyPipelineReason(pipelines []telemetryv1alpha1.MetricPipeline) string {
	// condTypes order defines the priority of negative conditions
	condTypes := []string{
		conditions.TypeConfigurationGenerated,
		conditions.TypeGatewayHealthy,
		conditions.TypeAgentHealthy,
	}

	if m.flowHealthProbingEnabled {
		condTypes = append(condTypes, conditions.TypeFlowHealthy)
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

func (m *metricComponentsChecker) determineConditionStatus(reason string) metav1.ConditionStatus {
	if reason == conditions.ReasonNoPipelineDeployed || reason == conditions.ReasonComponentsRunning || reason == conditions.ReasonTLSCertificateAboutToExpire {
		return metav1.ConditionTrue
	}
	return metav1.ConditionFalse
}

func (m *metricComponentsChecker) createMessageForReason(pipelines []telemetryv1alpha1.MetricPipeline, reason string) string {
	tlsAboutExpireMessage := m.firstTLSCertificateMessage(pipelines)
	if len(tlsAboutExpireMessage) > 0 {
		return tlsAboutExpireMessage
	}
	if reason != conditions.ReasonResourceBlocksDeletion {
		return conditions.MessageForMetricPipeline(reason)
	}

	return generateDeletionBlockedMessage(blockingResources{
		resourceType: "MetricPipelines",
		resourceNames: extslices.TransformFunc(pipelines, func(p telemetryv1alpha1.MetricPipeline) string {
			return p.Name
		}),
	})
}

func (m *metricComponentsChecker) firstTLSCertificateMessage(pipelines []telemetryv1alpha1.MetricPipeline) string {
	for _, p := range pipelines {
		tlsCertMsg := determineTLSCertMsg(p.Status.Conditions)
		if tlsCertMsg != "" {
			return tlsCertMsg
		}
	}
	return ""
}
