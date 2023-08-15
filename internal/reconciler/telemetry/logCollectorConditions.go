package telemetry

import (
	"context"
	"fmt"
	operatorV1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type logCollectorConditions struct {
	client        client.Client
	componentName types.NamespacedName
}

func NewLogCollectorConditions(client client.Client, componentName types.NamespacedName) *logCollectorConditions {
	return &logCollectorConditions{
		client:        client,
		componentName: componentName,
	}
}
func (l *logCollectorConditions) Name() string {
	return l.componentName.Name
}

func (l *logCollectorConditions) IsComponentHealthy(ctx context.Context) (*metav1.Condition, error) {
	var logpipelines v1alpha1.LogPipelineList
	err := l.client.List(ctx, &logpipelines)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to get all log pipelines while syncing conditions: %w", err)
	}
	if len(logpipelines.Items) == 0 {
		return l.buildTelemetryConditions(reconciler.ReasonNoPipelineDeployed), nil
	}

	// Try to get the status of the log collector via the pipelines
	status := l.validateLogPipeline(logpipelines.Items)
	return l.buildTelemetryConditions(status), nil

}

func (l *logCollectorConditions) Endpoints(ctx context.Context, config Config, endpoints operatorV1alpha1.Endpoints) (operatorV1alpha1.Endpoints, error) {
	return endpoints, nil
}

func (l *logCollectorConditions) validateLogPipeline(logPipeines []v1alpha1.LogPipeline) string {
	for _, l := range logPipeines {
		conditions := l.Status.Conditions
		if len(conditions) == 0 {
			return reconciler.ReasonFluentBitDSNotReady
		}
		if conditions[len(conditions)-1].Reason == reconciler.ReasonReferencedSecretMissingReason {
			return reconciler.ReasonReferencedSecretMissingReason
		}
		if conditions[len(conditions)-1].Type == v1alpha1.LogPipelinePending {
			return reconciler.ReasonFluentBitDSNotReady
		}
	}
	return reconciler.ReasonFluentBitDSReady
}

func (l *logCollectorConditions) buildTelemetryConditions(reason string) *metav1.Condition {
	if reason == reconciler.ReasonFluentBitDSReady || reason == reconciler.ReasonNoPipelineDeployed {
		return &metav1.Condition{
			Type:    reconciler.LogConditionType,
			Status:  reconciler.ConditionStatusTrue,
			Reason:  reason,
			Message: reconciler.Conditions[reason],
		}
	}
	return &metav1.Condition{
		Type:    reconciler.LogConditionType,
		Status:  reconciler.ConditionStatusFalse,
		Reason:  reason,
		Message: reconciler.Conditions[reason],
	}
}
