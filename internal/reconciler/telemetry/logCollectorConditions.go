package telemetry

import (
	"context"
	"fmt"
	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/kubernetes"
	"github.com/kyma-project/telemetry-manager/internal/reconciler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type logCollectorConditions struct {
	client        client.Client
	dsProber      kubernetes.DaemonSetProber
	componentName types.NamespacedName
}

type DaemonSetProber interface {
	Status(ctx context.Context, name types.NamespacedName) (string, error)
}

func NewLogCollectorConditions(client client.Client, dsProber kubernetes.DaemonSetProber, componentName types.NamespacedName) *logCollectorConditions {
	return &logCollectorConditions{
		client:        client,
		dsProber:      dsProber,
		componentName: componentName,
	}
}
func (l *logCollectorConditions) name() string {
	return l.componentName.Name
}

func (l *logCollectorConditions) isComponentHealthy(ctx context.Context) (*metav1.Condition, error) {
	var logpipelines v1alpha1.LogPipelineList
	err := l.client.List(ctx, &logpipelines)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to get all log pipelines while syncing conditions: %w", err)
	}
	if len(logpipelines.Items) == 0 {
		return buildTelemetryConditions(reconciler.ReasonNoPipelineDeployed), nil
	}
	if allLogPipelinesAreReady(logpipelines.Items) {
		return buildTelemetryConditions(reconciler.ReasonFluentBitDSReady), nil
	}

	if missingSecret(logpipelines.Items) {
		return buildTelemetryConditions(reconciler.ReasonReferencedSecretMissing), nil
	}

	status, err := l.dsProber.Status(ctx, l.componentName)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to get status of telemetry fluent bit: %w", err)
	}
	return buildTelemetryConditions(status), nil

}

func allLogPipelinesAreReady(logPipeines []v1alpha1.LogPipeline) bool {
	for _, l := range logPipeines {
		if l.Status.Conditions[0].Type == v1alpha1.LogPipelinePending {
			return false
		}
	}
	return true
}

func missingSecret(logPipeines []v1alpha1.LogPipeline) bool {
	for _, l := range logPipeines {
		if l.Status.Conditions[0].Type == v1alpha1.LogPipelinePending && l.Status.Conditions[0].Reason == reconciler.ReasonReferencedSecretMissingReason {
			return true
		}
	}
	return false
}

func buildTelemetryConditions(reason string) *metav1.Condition {
	if reason == reconciler.ReasonFluentBitDSReady || reason == reconciler.ReasonNoPipelineDeployed {
		return &metav1.Condition{
			Type:    "LogCollectorIsHealthy",
			Status:  "True",
			Reason:  reason,
			Message: conditions[reason],
		}
	}
	return &metav1.Condition{
		Type:    "LogCollectorIsHealthy",
		Status:  "False",
		Reason:  reason,
		Message: "",
	}
}
