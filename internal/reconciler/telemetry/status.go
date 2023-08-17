package telemetry

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

const (
	logComponentsHealthyConditionType    = "LogComponentsHealthy"
	traceComponentsHealthyConditionType  = "TraceComponentsHealthy"
	metricComponentsHealthyConditionType = "MetricComponentsHealthy"
)

//go:generate mockery --name ComponentHealthChecker --filename component_health_checker.go
type ComponentHealthChecker interface {
	Check(ctx context.Context) (*metav1.Condition, error)
}

func (r *Reconciler) updateStatus(ctx context.Context, obj *operatorv1alpha1.Telemetry) error {
	for _, checker := range r.healthCheckers {
		if err := r.updateComponentCondition(ctx, checker, obj); err != nil {
			return err
		}
	}
	return r.updateGatewayEndpoints(ctx, obj)
}

func (r *Reconciler) updateComponentCondition(ctx context.Context, checker ComponentHealthChecker, telemetry *operatorv1alpha1.Telemetry) error {
	newCondition, err := checker.Check(ctx)
	if err != nil {
		return fmt.Errorf("unable to check component: %w", err)
	}

	newCondition.ObservedGeneration = telemetry.GetGeneration()
	meta.SetStatusCondition(&telemetry.Status.Conditions, *newCondition)
	telemetry.Status.Status.State = r.nextState(telemetry)
	return r.serverSideApplyStatus(ctx, telemetry)
}

func (r *Reconciler) nextState(obj *operatorv1alpha1.Telemetry) operatorv1alpha1.State {
	conditions := obj.Status.Conditions
	var state operatorv1alpha1.State
	state = "Ready"
	for _, c := range conditions {
		if c.Status == metav1.ConditionFalse {
			state = "Warning"
		}
	}
	return state
}

func (r *Reconciler) updateGatewayEndpoints(ctx context.Context, telemetry *operatorv1alpha1.Telemetry) error {
	logf := log.FromContext(ctx)
	var metricEndpoints *operatorv1alpha1.OTLPEndpoints
	var err error

	if r.config.Metrics.Enabled {
		metricEndpoints, err = r.metricEndpoints(ctx, r.config, &telemetry.Status.Conditions)
		if err != nil {
			logf.Error(err, "Unable to update metric endpoints")
		}
	}

	traceEndpoints, err := r.traceEndpoints(ctx, r.config, &telemetry.Status.Conditions)
	if err != nil {
		logf.Error(err, "Unable to update trace endpoints")
	}

	telemetry.Status.GatewayEndpoints = operatorv1alpha1.GatewayEndpoints{
		Traces:  traceEndpoints,
		Metrics: metricEndpoints,
	}

	return r.serverSideApplyStatus(ctx, telemetry)
}

func (r *Reconciler) metricEndpoints(ctx context.Context, config Config, conditions *[]metav1.Condition) (*operatorv1alpha1.OTLPEndpoints, error) {
	var metricPipelines v1alpha1.MetricPipelineList
	err := r.Client.List(ctx, &metricPipelines)
	if err != nil {
		return nil, fmt.Errorf("failed to get all mertic pipelines while syncing conditions: %w", err)
	}
	if len(metricPipelines.Items) == 0 {
		return &operatorv1alpha1.OTLPEndpoints{}, nil
	}

	if !checkComponentConditionIsHealthy(metricComponentsHealthyConditionType, conditions) {
		return &operatorv1alpha1.OTLPEndpoints{}, nil
	}

	return makeOTLPEndpoints(config.Metrics.OTLPServiceName, config.Metrics.Namespace), nil
}

func (r *Reconciler) traceEndpoints(ctx context.Context, config Config, conditions *[]metav1.Condition) (*operatorv1alpha1.OTLPEndpoints, error) {
	var tracePipelines v1alpha1.TracePipelineList
	err := r.Client.List(ctx, &tracePipelines)
	if err != nil {
		return nil, fmt.Errorf("failed to get all trace pipelines while syncing conditions: %w", err)
	}
	if len(tracePipelines.Items) == 0 {
		return &operatorv1alpha1.OTLPEndpoints{}, nil
	}

	if !checkComponentConditionIsHealthy(traceComponentsHealthyConditionType, conditions) {
		return &operatorv1alpha1.OTLPEndpoints{}, nil
	}

	return makeOTLPEndpoints(config.Traces.OTLPServiceName, config.Traces.Namespace), nil
}

func makeOTLPEndpoints(serviceName, namespace string) *operatorv1alpha1.OTLPEndpoints {
	return &operatorv1alpha1.OTLPEndpoints{
		HTTP: fmt.Sprintf("http://%s.%s:%d", serviceName, namespace, ports.OTLPHTTP),
		GRPC: fmt.Sprintf("http://%s.%s:%d", serviceName, namespace, ports.OTLPGRPC),
	}

}

func checkComponentConditionIsHealthy(condType string, conditions *[]metav1.Condition) bool {
	for _, c := range *conditions {
		if c.Type == condType && c.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}
