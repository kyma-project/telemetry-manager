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

	// Try to get the status of the log collector via the pipelines
	status := validateLogPipeline(logpipelines.Items)
	if status != reconciler.ReasonFluentBitDSNotReady {
		return buildTelemetryConditions(status), nil
	}

	// if logpipelines are still pending check if they are in crashback loop or the pods of daemon set are starting up
	status, err = l.dsProber.Status(ctx, l.componentName)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to get status of telemetry fluent bit: %w", err)
	}
	return buildTelemetryConditions(status), nil

}

func validateLogPipeline(logPipeines []v1alpha1.LogPipeline) string {
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

func buildTelemetryConditions(reason string) *metav1.Condition {
	if reason == reconciler.ReasonFluentBitDSReady || reason == reconciler.ReasonNoPipelineDeployed {
		return &metav1.Condition{
			Type:    "LogCollectorIsHealthy",
			Status:  "True",
			Reason:  reason,
			Message: reconciler.Conditions[reason],
		}
	}
	return &metav1.Condition{
		Type:    "LogCollectorIsHealthy",
		Status:  "False",
		Reason:  reason,
		Message: reconciler.Conditions[reason],
	}
}
