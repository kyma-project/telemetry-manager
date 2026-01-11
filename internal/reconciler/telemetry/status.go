package telemetry

import (
	"context"
	"errors"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
)

type ComponentHealthChecker interface {
	Check(ctx context.Context, telemetryInDeletion bool) (*metav1.Condition, error)
}

func (r *Reconciler) updateStatus(ctx context.Context, telemetry *operatorv1beta1.Telemetry) error {
	telemetryInDeletion := !telemetry.GetDeletionTimestamp().IsZero()

	for _, checker := range r.enabledHealthCheckers() {
		if err := r.updateComponentCondition(ctx, checker, telemetry, telemetryInDeletion); err != nil {
			return fmt.Errorf("failed to update component condition: %w", err)
		}
	}

	r.updateOverallState(ctx, telemetry, telemetryInDeletion)

	if err := r.updateGatewayEndpoints(ctx, telemetry); err != nil {
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

func (r *Reconciler) updateComponentCondition(ctx context.Context, checker ComponentHealthChecker, telemetry *operatorv1beta1.Telemetry, telemetryInDeletion bool) error {
	newCondition, err := checker.Check(ctx, telemetryInDeletion)
	if err != nil {
		return fmt.Errorf("unable to check component: %w", err)
	}

	newCondition.ObservedGeneration = telemetry.GetGeneration()
	meta.SetStatusCondition(&telemetry.Status.Conditions, *newCondition)

	return nil
}

func (r *Reconciler) updateOverallState(ctx context.Context, telemetry *operatorv1beta1.Telemetry, telemetryInDeletion bool) {
	if telemetryInDeletion {
		// If the provided Telemetry CR is being deleted and dependent Telemetry CRs (LogPipeline, MetricPipeline, TracePipeline) are found, the state is set to "Warning" until they are removed from the cluster.
		// If dependent CRs are not found, the state is set to "Deleting"
		if r.dependentCRsFound(ctx) {
			telemetry.Status.State = operatorv1beta1.StateWarning
		} else {
			telemetry.Status.State = operatorv1beta1.StateDeleting
		}

		return
	}

	// Since LogPipeline, MetricPipeline, and TracePipeline have status conditions with positive polarity,
	// we can assume that the Telemetry Module is in the 'Ready' state if all conditions of dependent resources have the status 'True',
	// with the exception being the imminent expiration of the configured TLS certificate.
	if slices.ContainsFunc(telemetry.Status.Conditions, func(cond metav1.Condition) bool {
		return cond.Status == metav1.ConditionFalse || cond.Reason == conditions.ReasonTLSCertificateAboutToExpire
	}) {
		telemetry.Status.State = operatorv1beta1.StateWarning
	} else {
		telemetry.Status.State = operatorv1beta1.StateReady
	}
}

func (r *Reconciler) updateGatewayEndpoints(ctx context.Context, telemetry *operatorv1beta1.Telemetry) error {
	logEndpoints, err := r.logEndpoints(ctx, r.config)
	if err != nil {
		return fmt.Errorf("failed to get log endpoints: %w", err)
	}

	traceEndpoints, err := r.traceEndpoints(ctx, r.config)
	if err != nil {
		return fmt.Errorf("failed to get trace endpoints: %w", err)
	}

	metricEndpoints, err := r.metricEndpoints(ctx, r.config)
	if err != nil {
		return fmt.Errorf("failed to get metric endpoints: %w", err)
	}

	telemetry.Status.Endpoints = operatorv1beta1.GatewayEndpoints{
		Logs:    logEndpoints,
		Traces:  traceEndpoints,
		Metrics: metricEndpoints,
	}

	return nil
}

func (r *Reconciler) logEndpoints(ctx context.Context, config Config) (*operatorv1beta1.OTLPEndpoints, error) {
	pushEndpoint := types.NamespacedName{
		Name:      otelcollector.LogOTLPServiceName,
		Namespace: config.TargetNamespace(),
	}

	svcExists, err := r.checkServiceExists(ctx, pushEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to check if log service endpoints exist: %w", err)
	}

	// Service has not been created yet, so we return nil.
	if !svcExists {
		return nil, nil //nolint:nilnil //it is ok in this context, even if it is not go idiomatic
	}

	return makeOTLPEndpoints(pushEndpoint.Name, pushEndpoint.Namespace), nil
}

func (r *Reconciler) traceEndpoints(ctx context.Context, config Config) (*operatorv1beta1.OTLPEndpoints, error) {
	pushEndpoint := types.NamespacedName{
		Name:      otelcollector.TraceOTLPServiceName,
		Namespace: config.TargetNamespace(),
	}

	svcExists, err := r.checkServiceExists(ctx, pushEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to check if trace service endpoints exist: %w", err)
	}

	// Service has not been created yet, so we return nil.
	if !svcExists {
		return nil, nil //nolint:nilnil //it is ok in this context, even if it is not go idiomatic
	}

	return makeOTLPEndpoints(pushEndpoint.Name, pushEndpoint.Namespace), nil
}

func (r *Reconciler) metricEndpoints(ctx context.Context, config Config) (*operatorv1beta1.OTLPEndpoints, error) {
	pushEndpoint := types.NamespacedName{
		Name:      otelcollector.MetricOTLPServiceName,
		Namespace: config.TargetNamespace(),
	}

	svcExists, err := r.checkServiceExists(ctx, pushEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to check if metric service endpoints exist: %w", err)
	}

	// Service has not been created yet, so we return nil.
	if !svcExists {
		return nil, nil //nolint:nilnil //it is ok in this context, even if it is not go idiomatic
	}

	return makeOTLPEndpoints(pushEndpoint.Name, pushEndpoint.Namespace), nil
}

func (r *Reconciler) checkServiceExists(ctx context.Context, svcName types.NamespacedName) (bool, error) {
	var service corev1.Service

	err := r.Get(ctx, svcName, &service)
	if err != nil {
		// If the pipeline is not configured with OTLP input, gateway won't be deployed. In such case we can safely return error
		if client.IgnoreNotFound(err) != nil {
			return false, fmt.Errorf("failed to get log-otlp service: %w", err)
		}

		return false, nil
	}

	return true, nil
}

func makeOTLPEndpoints(serviceName, namespace string) *operatorv1beta1.OTLPEndpoints {
	return &operatorv1beta1.OTLPEndpoints{
		HTTP: fmt.Sprintf("http://%s.%s:%d", serviceName, namespace, ports.OTLPHTTP),
		GRPC: fmt.Sprintf("http://%s.%s:%d", serviceName, namespace, ports.OTLPGRPC),
	}
}

func (r *Reconciler) dependentCRsFound(ctx context.Context) bool {
	return r.resourcesExist(ctx, &telemetryv1beta1.LogPipelineList{}) ||
		r.resourcesExist(ctx, &telemetryv1beta1.MetricPipelineList{}) ||
		r.resourcesExist(ctx, &telemetryv1beta1.TracePipelineList{})
}

func (r *Reconciler) resourcesExist(ctx context.Context, list client.ObjectList) bool {
	if err := r.List(ctx, list); err != nil {
		// no kind found
		return errors.Is(err, &meta.NoKindMatchError{})
	}

	return meta.LenList(list) > 0
}
