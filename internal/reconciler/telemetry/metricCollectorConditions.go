package telemetry

import (
	"context"
	"fmt"
	operatorV1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	"github.com/kyma-project/telemetry-manager/internal/reconciler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type metricCollectorConditions struct {
	client        client.Client
	componentName types.NamespacedName
}

func NewMetricCollector(client client.Client, componentName types.NamespacedName) *metricCollectorConditions {
	return &metricCollectorConditions{
		client:        client,
		componentName: componentName,
	}
}

func (m *metricCollectorConditions) Name() string {
	return m.componentName.Name
}

func (m *metricCollectorConditions) IsComponentHealthy(ctx context.Context) (*metav1.Condition, error) {
	metricPipelines, err := m.getPipelines(ctx)
	if err != nil {
		return &metav1.Condition{}, err
	}

	if len(metricPipelines.Items) == 0 {
		return m.buildTelemetryConditions(reconciler.ReasonNoPipelineDeployed), nil
	}

	// Try to get the status of the metric collector via the pipelines
	status := m.validateMetricPipeline(metricPipelines.Items)
	return m.buildTelemetryConditions(status), nil
}

func (m *metricCollectorConditions) Endpoints(ctx context.Context, config Config, endpoints operatorV1alpha1.Endpoints) (operatorV1alpha1.Endpoints, error) {
	metricPipelines, err := m.getPipelines(ctx)
	metricEndpoints := operatorV1alpha1.MetricEndpoints{}
	if err != nil {
		return endpoints, err
	}
	if len(metricPipelines.Items) == 0 {
		endpoints.Metrics = metricEndpoints
		return endpoints, nil
	}
	metricEndpoints.HTTP = fmt.Sprintf("http://%s.%s:%d", config.MetricConfig.ServiceName, config.MetricConfig.Namespace, ports.OTLPHTTP)
	metricEndpoints.GRPC = fmt.Sprintf("http://%s.%s:%d", config.MetricConfig.ServiceName, config.MetricConfig.Namespace, ports.OTLPGRPC)
	endpoints.Metrics = metricEndpoints

	return endpoints, nil
}

func (m *metricCollectorConditions) getPipelines(ctx context.Context) (v1alpha1.MetricPipelineList, error) {
	var metricPipelines v1alpha1.MetricPipelineList
	err := m.client.List(ctx, &metricPipelines)
	if err != nil {
		return v1alpha1.MetricPipelineList{}, fmt.Errorf("failed to get all mertic pipelines while syncing conditions: %w", err)
	}
	return metricPipelines, nil
}

func (m *metricCollectorConditions) validateMetricPipeline(metricPipelines []v1alpha1.MetricPipeline) string {
	for _, m := range metricPipelines {
		conditions := m.Status.Conditions
		if len(conditions) == 0 {
			return reconciler.ReasonMetricGatewayDeploymentNotReady
		}
		if conditions[len(conditions)-1].Reason == reconciler.ReasonReferencedSecretMissingReason {
			return reconciler.ReasonReferencedSecretMissingReason
		}
		if conditions[len(conditions)-1].Type == v1alpha1.MetricPipelinePending {
			return reconciler.ReasonMetricGatewayDeploymentNotReady
		}
	}
	return reconciler.ReasonMetricGatewayDeploymentReady
}

func (m *metricCollectorConditions) buildTelemetryConditions(reason string) *metav1.Condition {
	if reason == reconciler.ReasonMetricGatewayDeploymentReady || reason == reconciler.ReasonNoPipelineDeployed {
		return &metav1.Condition{
			Type:    reconciler.MetricConditionType,
			Status:  reconciler.ConditionStatusTrue,
			Reason:  reason,
			Message: reconciler.Conditions[reason],
		}
	}
	return &metav1.Condition{
		Type:    reconciler.MetricConditionType,
		Status:  reconciler.ConditionStatusFalse,
		Reason:  reason,
		Message: reconciler.Conditions[reason],
	}
}
