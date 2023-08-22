package telemetry

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func (r *Reconciler) updateStatus(ctx context.Context, telemetry *operatorv1alpha1.Telemetry) error {
	for _, checker := range r.healthCheckers {
		if err := r.updateComponentCondition(ctx, checker, telemetry); err != nil {
			return fmt.Errorf("failed to update component condition: %w", err)
		}
	}

	if err := r.updateGatewayEndpoints(ctx, telemetry); err != nil {
		return fmt.Errorf("failed to update gateway endpoints: %w", err)
	}

	if err := r.checkDependentTelemetryCRs(ctx, telemetry); err != nil {
		return fmt.Errorf("failed to check if telemetry is being deleted: %w", err)
	}

	return nil
}

func (r *Reconciler) checkDependentTelemetryCRs(ctx context.Context, telemetry *operatorv1alpha1.Telemetry) error {
	instanceIsBeingDeleted := !telemetry.GetDeletionTimestamp().IsZero()
	if instanceIsBeingDeleted &&
		telemetry.Status.State != operatorv1alpha1.StateDeleting {
		if r.dependentTelemetryCRsFound(ctx) {
			r.updateStatusState(ctx, telemetry, operatorv1alpha1.StateError)
		}
		r.updateStatusState(ctx, telemetry, operatorv1alpha1.StateDeleting)
	}

	return nil
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
		metricEndpoints, err = r.metricEndpoints(ctx, r.config, telemetry.Status.Conditions)
		if err != nil {
			logf.Error(err, "Unable to update metric endpoints")
		}
	}

	traceEndpoints, err := r.traceEndpoints(ctx, r.config, telemetry.Status.Conditions)
	if err != nil {
		logf.Error(err, "Unable to update trace endpoints")
	}

	telemetry.Status.GatewayEndpoints = operatorv1alpha1.GatewayEndpoints{
		Traces:  traceEndpoints,
		Metrics: metricEndpoints,
	}

	return r.serverSideApplyStatus(ctx, telemetry)
}

func (r *Reconciler) metricEndpoints(ctx context.Context, config Config, conditions []metav1.Condition) (*operatorv1alpha1.OTLPEndpoints, error) {
	var metricPipelines telemetryv1alpha1.MetricPipelineList
	err := r.Client.List(ctx, &metricPipelines)
	if err != nil {
		return nil, fmt.Errorf("failed to get all mertic pipelines while syncing conditions: %w", err)
	}
	if len(metricPipelines.Items) == 0 {
		return &operatorv1alpha1.OTLPEndpoints{}, nil
	}

	if !meta.IsStatusConditionTrue(conditions, metricComponentsHealthyConditionType) {
		return &operatorv1alpha1.OTLPEndpoints{}, nil
	}

	return makeOTLPEndpoints(config.Metrics.OTLPServiceName, config.Metrics.Namespace), nil
}

func (r *Reconciler) traceEndpoints(ctx context.Context, config Config, conditions []metav1.Condition) (*operatorv1alpha1.OTLPEndpoints, error) {
	var tracePipelines telemetryv1alpha1.TracePipelineList
	err := r.Client.List(ctx, &tracePipelines)
	if err != nil {
		return nil, fmt.Errorf("failed to get all trace pipelines while syncing conditions: %w", err)
	}
	if len(tracePipelines.Items) == 0 {
		return &operatorv1alpha1.OTLPEndpoints{}, nil
	}

	if !meta.IsStatusConditionTrue(conditions, traceComponentsHealthyConditionType) {
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

func (r *Reconciler) updateStatusState(ctx context.Context, telemetry *operatorv1alpha1.Telemetry, newState operatorv1alpha1.State) error {
	telemetry.Status.State = newState

	if err := r.serverSideApplyStatus(ctx, telemetry); err != nil {
		r.Event(telemetry, "Warning", "ErrorUpdatingStatus", fmt.Sprintf("updating state to %v", string(newState)))
		return fmt.Errorf("error while updating status %s to: %w", newState, err)
	}

	r.Event(telemetry, "Normal", "StatusUpdated", fmt.Sprintf("updating state to %v", string(newState)))
	return nil
}

func (r *Reconciler) serverSideApplyStatus(ctx context.Context, obj client.Object) error {
	obj.SetManagedFields(nil)
	obj.SetResourceVersion("")
	return r.Status().Patch(ctx, obj, client.Apply,
		&client.SubResourcePatchOptions{PatchOptions: client.PatchOptions{FieldManager: fieldOwner}})
}

func (r *Reconciler) dependentTelemetryCRsFound(ctx context.Context) bool {
	return r.resourcesExist(ctx, &telemetryv1alpha1.LogParserList{}) ||
		r.resourcesExist(ctx, &telemetryv1alpha1.LogPipelineList{}) ||
		r.resourcesExist(ctx, &telemetryv1alpha1.MetricPipelineList{}) ||
		r.resourcesExist(ctx, &telemetryv1alpha1.TracePipelineList{})
}

func (r *Reconciler) resourcesExist(ctx context.Context, list client.ObjectList) bool {
	if err := r.List(ctx, list); err != nil {
		// no kind found
		if _, ok := err.(*meta.NoKindMatchError); ok {
			return false
		}
		return true
	}
	return meta.LenList(list) > 0
}
