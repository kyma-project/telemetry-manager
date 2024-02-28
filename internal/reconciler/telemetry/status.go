package telemetry

import (
	"context"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

//go:generate mockery --name ComponentHealthChecker --filename component_health_checker.go
type ComponentHealthChecker interface {
	Check(ctx context.Context, telemetryInDeletion bool) (*metav1.Condition, error)
}

func (r *Reconciler) updateStatus(ctx context.Context, telemetry *operatorv1alpha1.Telemetry) error {
	telemetryInDeletion := !telemetry.GetDeletionTimestamp().IsZero()

	for _, checker := range r.enabledHealthCheckers() {
		if err := r.updateComponentCondition(ctx, checker, telemetry, telemetryInDeletion); err != nil {
			return fmt.Errorf("failed to update component condition: %w", err)
		}
	}

	r.updateOverallState(ctx, telemetry, telemetryInDeletion)

	if err := r.updateGatewayEndpoints(ctx, telemetry, telemetryInDeletion); err != nil {
		return fmt.Errorf("failed to update gateway endpoints: %w", err)
	}

	if err := r.Status().Update(ctx, telemetry); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	return nil
}

func (r *Reconciler) enabledHealthCheckers() []ComponentHealthChecker {
	return []ComponentHealthChecker{r.healthCheckers.logs, r.healthCheckers.metrics, r.healthCheckers.traces}
}

func (r *Reconciler) updateComponentCondition(ctx context.Context, checker ComponentHealthChecker, telemetry *operatorv1alpha1.Telemetry, telemetryInDeletion bool) error {
	newCondition, err := checker.Check(ctx, telemetryInDeletion)
	if err != nil {
		return fmt.Errorf("unable to check component: %w", err)
	}

	newCondition.ObservedGeneration = telemetry.GetGeneration()
	meta.SetStatusCondition(&telemetry.Status.Conditions, *newCondition)

	return nil
}

func (r *Reconciler) updateOverallState(ctx context.Context, telemetry *operatorv1alpha1.Telemetry, telemetryInDeletion bool) {
	if telemetryInDeletion {
		// If the provided Telemetry CR is being deleted and dependent Telemetry CRs (LogPipeline, LogParser, MetricPipeline, TracePipeline) are found, the state is set to "Warning" until they are removed from the cluster.
		// If dependent CRs are not found, the state is set to "Deleting"
		if r.dependentCRsFound(ctx) {
			telemetry.Status.State = operatorv1alpha1.StateWarning
		} else {
			telemetry.Status.State = operatorv1alpha1.StateDeleting
		}
		return
	}

	// Since LogPipeline, MetricPipeline, and TracePipeline have status conditions with positive polarity,
	// we can assume that the Telemetry Module is in the 'Ready' state if all conditions of dependent resources have the status 'True.'
	if slices.ContainsFunc(telemetry.Status.Conditions, func(cond metav1.Condition) bool {
		return cond.Status == metav1.ConditionFalse
	}) {
		telemetry.Status.State = operatorv1alpha1.StateWarning
	} else {
		telemetry.Status.State = operatorv1alpha1.StateReady
	}
}

func (r *Reconciler) updateGatewayEndpoints(ctx context.Context, telemetry *operatorv1alpha1.Telemetry, telemetryInDeletion bool) error {
	traceEndpoints, err := r.traceEndpoints(ctx, r.config, telemetryInDeletion)
	if err != nil {
		return fmt.Errorf("failed to get trace endpoints: %w", err)
	}

	metricEndpoints, err := r.metricEndpoints(ctx, r.config, telemetryInDeletion)
	if err != nil {
		return fmt.Errorf("failed to get metric endpoints: %w", err)
	}

	telemetry.Status.GatewayEndpoints = operatorv1alpha1.GatewayEndpoints{
		Traces:  traceEndpoints,
		Metrics: metricEndpoints,
	}

	return nil
}

func (r *Reconciler) traceEndpoints(ctx context.Context, config Config, telemetryInDeletion bool) (*operatorv1alpha1.OTLPEndpoints, error) {
	cond, err := r.healthCheckers.traces.Check(ctx, telemetryInDeletion)
	if err != nil {
		return nil, fmt.Errorf("failed to check trace components: %w", err)
	}
	if cond.Status != metav1.ConditionTrue || cond.Reason != conditions.ReasonTraceComponentsRunning {
		return nil, nil //nolint:nilnil //it is ok in this context, even if it is not go idiomatic
	}

	return makeOTLPEndpoints(config.Traces.OTLPServiceName, config.Traces.Namespace), nil
}

func (r *Reconciler) metricEndpoints(ctx context.Context, config Config, telemetryInDeletion bool) (*operatorv1alpha1.OTLPEndpoints, error) {
	cond, err := r.healthCheckers.metrics.Check(ctx, telemetryInDeletion)
	if err != nil {
		return nil, fmt.Errorf("failed to check trace components: %w", err)
	}
	if cond.Status != metav1.ConditionTrue || cond.Reason != conditions.ReasonMetricComponentsRunning {
		return nil, nil //nolint:nilnil //it is ok in this context, even if it is not go idiomatic
	}

	return makeOTLPEndpoints(config.Metrics.OTLPServiceName, config.Metrics.Namespace), nil
}

func makeOTLPEndpoints(serviceName, namespace string) *operatorv1alpha1.OTLPEndpoints {
	return &operatorv1alpha1.OTLPEndpoints{
		HTTP: fmt.Sprintf("http://%s.%s:%d", serviceName, namespace, ports.OTLPHTTP),
		GRPC: fmt.Sprintf("http://%s.%s:%d", serviceName, namespace, ports.OTLPGRPC),
	}
}

func (r *Reconciler) dependentCRsFound(ctx context.Context) bool {
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
