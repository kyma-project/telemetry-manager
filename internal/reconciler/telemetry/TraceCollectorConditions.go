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

type traceCollectorConditions struct {
	client        client.Client
	dpProber      kubernetes.DeploymentProber
	componentName types.NamespacedName
}

func NewTraceCollector(client client.Client, dpProber kubernetes.DeploymentProber, componentName types.NamespacedName) *traceCollectorConditions {
	return &traceCollectorConditions{
		client:        client,
		dpProber:      dpProber,
		componentName: componentName,
	}
}

func (m *traceCollectorConditions) name() string {
	return m.componentName.Name
}

func (m *traceCollectorConditions) isComponentHealthy(ctx context.Context) (*metav1.Condition, error) {
	var tracePipelines v1alpha1.TracePipelineList
	err := m.client.List(ctx, &tracePipelines)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to get all trace pipelines while syncing conditions: %w", err)
	}
	if len(tracePipelines.Items) == 0 {
		return buildTelemetryConditions(reconciler.ReasonNoPipelineDeployed), nil
	}
	if allTracePipelinesAreReady(tracePipelines.Items) {
		return buildTelemetryConditions(reconciler.ReasonTraceCollectorDeploymentReady), nil
	}

	status, err := m.dpProber.Status(ctx, m.componentName)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to get status of telemetry fluent bit: %w", err)
	}
	return buildTelemetryConditions(status), nil
}

func allTracePipelinesAreReady(tracePipelines []v1alpha1.TracePipeline) bool {
	for _, l := range tracePipelines {
		if l.Status.Conditions[0].Type == v1alpha1.TracePipelinePending {
			return false
		}
	}
	return true
}
