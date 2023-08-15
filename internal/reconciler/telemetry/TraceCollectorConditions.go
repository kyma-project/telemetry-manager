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

type traceCollectorConditions struct {
	client        client.Client
	componentName types.NamespacedName
}

func NewTraceCollector(client client.Client, componentName types.NamespacedName) *traceCollectorConditions {
	return &traceCollectorConditions{
		client:        client,
		componentName: componentName,
	}
}

func (t *traceCollectorConditions) name() string {
	return t.componentName.Name
}

func (t *traceCollectorConditions) isComponentHealthy(ctx context.Context) (*metav1.Condition, error) {
	tracePipelines, err := t.getPipelines(ctx)
	if err != nil {
		return &metav1.Condition{}, err
	}
	if len(tracePipelines.Items) == 0 {
		return t.buildTelemetryConditions(reconciler.ReasonNoPipelineDeployed), nil
	}

	// Try to get the status of the trace collector via the pipelines
	status := t.validateTracePipeline(tracePipelines.Items)
	return t.buildTelemetryConditions(status), nil

}

func (t *traceCollectorConditions) endpoints(ctx context.Context, config Config, endpoints operatorV1alpha1.Endpoints) (operatorV1alpha1.Endpoints, error) {
	tracePipelines, err := t.getPipelines(ctx)
	traceEndpoints := operatorV1alpha1.TraceEndpoints{}
	if err != nil {
		return endpoints, err
	}
	if len(tracePipelines.Items) == 0 {
		endpoints.Traces = traceEndpoints
		return endpoints, nil
	}
	traceEndpoints.HTTP = fmt.Sprintf("http://%s.%s:%d", config.TraceConfig.ServiceName, config.TraceConfig.Namespace, ports.OTLPHTTP)
	traceEndpoints.GRPC = fmt.Sprintf("http://%s.%s:%d", config.TraceConfig.ServiceName, config.TraceConfig.Namespace, ports.OTLPGRPC)
	endpoints.Traces = traceEndpoints

	return endpoints, nil
}

func (t *traceCollectorConditions) getPipelines(ctx context.Context) (v1alpha1.TracePipelineList, error) {
	var tracePipelines v1alpha1.TracePipelineList
	err := t.client.List(ctx, &tracePipelines)
	if err != nil {
		return v1alpha1.TracePipelineList{}, fmt.Errorf("failed to get all trace pipelines while syncing conditions: %w", err)
	}
	return tracePipelines, nil
}

func (t *traceCollectorConditions) validateTracePipeline(tracePipeines []v1alpha1.TracePipeline) string {
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

func (t *traceCollectorConditions) buildTelemetryConditions(reason string) *metav1.Condition {
	if reason == reconciler.ReasonTraceCollectorDeploymentReady || reason == reconciler.ReasonNoPipelineDeployed {
		return &metav1.Condition{
			Type:    "TraceCollectorIsHealthy",
			Status:  "True",
			Reason:  reason,
			Message: reconciler.Conditions[reason],
		}
	}
	return &metav1.Condition{
		Type:    "TraceCollectorIsHealthy",
		Status:  "False",
		Reason:  reason,
		Message: reconciler.Conditions[reason],
	}
}
