package telemetry

import (
	"context"
	"fmt"
	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	"github.com/kyma-project/telemetry-manager/internal/reconciler"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

//go:generate mockery --name ConditionsProber --filename conditions_prober.go
type componentHealthChecker interface {
	check(ctx context.Context) (*metav1.Condition, error)
}

//type components struct {
//	list map[string]componentHealthChecker
//}

//func New(lc componentHealthChecker, tc componentHealthChecker, mc componentHealthChecker) *components {
//	l  := make(map[string]componentHealthChecker)
//	l["Log Components"] = lc
//	l["Trace Compoennts"] = tc
//	l["Metric Components"] = mc
//
//	return &components{
//		list: l
//	}
//}

func (r *Reconciler) updateStatus(ctx context.Context, obj *operatorv1alpha1.Telemetry) error {
	for component, healthChecker := range r.healthCheckers {
		if err := r.updateConditions(ctx, component, healthChecker, obj); err != nil {
			return err
		}
	}
	if err := r.updateEndpoints(ctx, obj); err != nil {
		return err
	}
	if err := r.updateState(ctx, obj); err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) updateState(ctx context.Context, obj *operatorv1alpha1.Telemetry) error {
	conditions := obj.Status.Conditions
	var state operatorv1alpha1.State
	state = "Ready"
	for _, c := range conditions {
		if c.Status == reconciler.ConditionStatusFalse {
			state = "Warning"
		}
	}
	obj.Status.State = state
	return r.serverSideApplyStatus(ctx, obj)
}

func (r *Reconciler) updateConditions(ctx context.Context, compName string, cp componentHealthChecker, obj *operatorv1alpha1.Telemetry) error {
	logf := log.FromContext(ctx)
	logf.Info(fmt.Sprintf("Updating condition for: %s", compName))
	conditions := &obj.Status.Conditions
	newCondition, err := cp.check(ctx)
	if err != nil {
		return fmt.Errorf("unable to update conditions for: %v, %w", compName, err)
	}
	logf.Info(fmt.Sprintf("Got condition: %+v\n", newCondition))

	operatorStatus := operatorv1alpha1.Status{State: "Ready"}
	for _, c := range *conditions {
		if c.Status == "False" {
			operatorStatus.State = "Warning"
		}
	}
	newCondition.ObservedGeneration = obj.GetGeneration()

	meta.SetStatusCondition(&obj.Status.Conditions, *newCondition)
	obj.Status.Status = operatorStatus
	return r.serverSideApplyStatus(ctx, obj)
}

func (r *Reconciler) updateEndpoints(ctx context.Context, obj *operatorv1alpha1.Telemetry) error {
	logf := log.FromContext(ctx)
	metricEndpoints, err := r.metricEndpoints(ctx, r.TelemetryConfig)
	if err != nil {
		logf.Error(err, "Unable to update metric endpoints")
	}
	traceEndpoints, err := r.traceEndpoints(ctx, r.TelemetryConfig)
	if err != nil {
		logf.Error(err, "Unable to update trace endpoints")
	}

	obj.Status.Endpoints = operatorv1alpha1.Endpoints{
		Traces:  traceEndpoints,
		Metrics: metricEndpoints,
	}
	return r.serverSideApplyStatus(ctx, obj)
}

func (r *Reconciler) metricEndpoints(ctx context.Context, config Config) (*operatorv1alpha1.OTLPEndpoints, error) {
	var metricPipelines v1alpha1.MetricPipelineList
	err := r.Client.List(ctx, &metricPipelines)
	if err != nil {
		return nil, fmt.Errorf("failed to get all mertic pipelines while syncing conditions: %w", err)
	}
	if len(metricPipelines.Items) == 0 {
		return nil, nil
	}

	return makeOTLPEndpoints(config.MetricConfig.ServiceName, config.MetricConfig.Namespace), nil
}

func (r *Reconciler) traceEndpoints(ctx context.Context, config Config) (*operatorv1alpha1.OTLPEndpoints, error) {
	var tracePipelines v1alpha1.TracePipelineList
	err := r.Client.List(ctx, &tracePipelines)
	if err != nil {
		return nil, fmt.Errorf("failed to get all trace pipelines while syncing conditions: %w", err)
	}
	if len(tracePipelines.Items) == 0 {
		return nil, nil
	}

	return makeOTLPEndpoints(config.TraceConfig.ServiceName, config.TraceConfig.Namespace), nil
}

func makeOTLPEndpoints(serviceName, namespace string) *operatorv1alpha1.OTLPEndpoints {
	return &operatorv1alpha1.OTLPEndpoints{
		HTTP: fmt.Sprintf("http://%s.%s:%d", serviceName, namespace, ports.OTLPHTTP),
		GRPC: fmt.Sprintf("http://%s.%s:%d", serviceName, namespace, ports.OTLPGRPC),
	}

}
