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

type metricCollectorConditions struct {
	client        client.Client
	dpProber      kubernetes.DeploymentProber
	componentName types.NamespacedName
}
type DeploymentProber interface {
	Status(ctx context.Context, name types.NamespacedName) (string, error)
}

func NewMetricCollector(client client.Client, dpProber kubernetes.DeploymentProber, componentName types.NamespacedName) *metricCollectorConditions {
	return &metricCollectorConditions{
		client:        client,
		dpProber:      dpProber,
		componentName: componentName,
	}
}

func (m *metricCollectorConditions) name() string {
	return m.componentName.Name
}

func (m *metricCollectorConditions) isComponentHealthy(ctx context.Context) (*metav1.Condition, error) {
	var metricPipelines v1alpha1.MetricPipelineList
	err := m.client.List(ctx, &metricPipelines)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to get all metric pipelines while syncing conditions: %w", err)
	}
	if len(metricPipelines.Items) == 0 {
		return buildTelemetryConditions(reconciler.ReasonNoPipelineDeployed), nil
	}
	if allMetricPipelinesAreReady(metricPipelines.Items) {
		return buildTelemetryConditions(reconciler.ReasonMetricGatewayDeploymentReady), nil
	}

	status, err := m.dpProber.Status(ctx, m.componentName)
	if err != nil {
		return &metav1.Condition{}, fmt.Errorf("failed to get status of telemetry fluent bit: %w", err)
	}
	return buildTelemetryConditions(status), nil
}

func allMetricPipelinesAreReady(metricPipeines []v1alpha1.MetricPipeline) bool {
	for _, l := range metricPipeines {
		if l.Status.Conditions[0].Type == v1alpha1.MetricPipelinePending {
			return false
		}
	}
	return true
}
