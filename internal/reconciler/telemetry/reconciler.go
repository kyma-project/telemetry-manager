package telemetry

import (
	"context"
	"fmt"
	"time"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/kubernetes"
	"github.com/kyma-project/telemetry-manager/internal/webhookcert"
)

const (
	requeueInterval = time.Second * 10
	finalizer       = "telemetry.kyma-project.io/finalizer"
	fieldOwner      = "telemetry.kyma-project.io/owner"
)

type Config struct {
	Traces  TracesConfig
	Metrics MetricsConfig
	Webhook WebhookConfig
}

type TracesConfig struct {
	OTLPServiceName string
	Namespace       string
}

type MetricsConfig struct {
	Enabled         bool
	OTLPServiceName string
	Namespace       string
}

type WebhookConfig struct {
	Enabled    bool
	CertConfig webhookcert.Config
}

type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
	*rest.Config
	// EventRecorder for creating k8s events
	record.EventRecorder
	config         Config
	healthCheckers []ComponentHealthChecker
}

func NewReconciler(client client.Client, scheme *runtime.Scheme, eventRecorder record.EventRecorder, config Config) *Reconciler {
	healthCheckers := []ComponentHealthChecker{
		&logComponentsChecker{client: client},
		&traceComponentsChecker{client: client},
	}
	if config.Metrics.Enabled {
		healthCheckers = append(healthCheckers, &metricComponentsChecker{client: client})
	}

	return &Reconciler{
		Client:         client,
		Scheme:         scheme,
		EventRecorder:  eventRecorder,
		config:         config,
		healthCheckers: healthCheckers,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	objectInstance := operatorv1alpha1.Telemetry{}
	if err := r.Client.Get(ctx, req.NamespacedName, &objectInstance); err != nil {
		logger.Info(req.NamespacedName.String() + " got deleted!")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.updateStatus(ctx, &objectInstance); err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	instanceIsBeingDeleted := !objectInstance.GetDeletionTimestamp().IsZero()

	// Check if deletionTimestamp is set, retry until it gets deleted
	status := getStatusFromTelemetry(&objectInstance)

	if instanceIsBeingDeleted &&
		status.State != operatorv1alpha1.StateDeleting {
		if r.customResourceExist(ctx) {
			// there are some resources still in use update status and retry
			return ctrl.Result{Requeue: true}, r.setStatusForObjectInstance(ctx, &objectInstance, status.WithState(operatorv1alpha1.StateError))
		}
		// if the status is not yet set to deleting, also update the status
		return ctrl.Result{}, r.setStatusForObjectInstance(ctx, &objectInstance, status.WithState(operatorv1alpha1.StateDeleting))
	}

	// add finalizer if not present
	if controllerutil.AddFinalizer(&objectInstance, finalizer) {
		return ctrl.Result{}, r.serverSideApply(ctx, &objectInstance)
	}

	if err := r.reconcileWebhook(ctx, &objectInstance); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile webhook: %w", err)
	}

	switch status.State {
	case "":
		return ctrl.Result{}, r.HandleInitialState(ctx, &objectInstance)
	case operatorv1alpha1.StateProcessing:
		return ctrl.Result{Requeue: true}, r.HandleProcessingState(ctx, &objectInstance)
	case operatorv1alpha1.StateDeleting:
		return ctrl.Result{Requeue: true}, r.HandleDeletingState(ctx, &objectInstance)
	case operatorv1alpha1.StateError:
		return ctrl.Result{Requeue: true}, r.HandleErrorState(ctx, &objectInstance)
	case operatorv1alpha1.StateReady:
		return ctrl.Result{RequeueAfter: requeueInterval}, r.HandleReadyState(ctx, &objectInstance)
	case operatorv1alpha1.StateWarning:
		return ctrl.Result{RequeueAfter: requeueInterval}, r.HandleErrorState(ctx, &objectInstance)

	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) reconcileWebhook(ctx context.Context, telemetry *operatorv1alpha1.Telemetry) error {
	if !r.config.Webhook.Enabled {
		return nil
	}

	if !telemetry.DeletionTimestamp.IsZero() {
		return nil
	}

	if err := webhookcert.EnsureCertificate(ctx, r.Client, r.config.Webhook.CertConfig); err != nil {
		return fmt.Errorf("failed to reconcile webhook: %w", err)
	}

	var secret corev1.Secret
	if err := r.Get(ctx, r.config.Webhook.CertConfig.CASecretName, &secret); err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}
	if err := controllerutil.SetOwnerReference(telemetry, &secret, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference for secret: %w", err)
	}
	if err := kubernetes.CreateOrUpdateSecret(ctx, r.Client, &secret); err != nil {
		return fmt.Errorf("failed to update secret: %w", err)
	}

	var webhook admissionv1.ValidatingWebhookConfiguration
	if err := r.Get(ctx, r.config.Webhook.CertConfig.WebhookName, &webhook); err != nil {
		return fmt.Errorf("failed to get webhook: %w", err)
	}
	if err := kubernetes.CreateOrUpdateValidatingWebhookConfiguration(ctx, r.Client, &webhook); err != nil {
		return fmt.Errorf("failed to update webhook: %w", err)
	}

	return nil
}

func (r *Reconciler) deleteWebhook(ctx context.Context) error {
	webhook := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: r.config.Webhook.CertConfig.WebhookName.Name,
		},
	}

	return r.Delete(ctx, webhook)
}

// HandleReadyState checks for the consistency of reconciled resource, by verifying the underlying resources.
func (r *Reconciler) HandleReadyState(_ context.Context, _ *operatorv1alpha1.Telemetry) error {
	return nil
}

// HandleErrorState handles error recovery for the reconciled resource.
func (r *Reconciler) HandleErrorState(ctx context.Context, objectInstance *operatorv1alpha1.Telemetry) error {
	status := getStatusFromTelemetry(objectInstance)

	// set eventual state to Ready - if no errors were found
	return r.setStatusForObjectInstance(ctx, objectInstance, status.
		WithState(operatorv1alpha1.StateReady).
		WithInstallConditionStatus(metav1.ConditionTrue, objectInstance.GetGeneration()))
}

// HandleInitialState bootstraps state handling for the reconciled resource.
func (r *Reconciler) HandleInitialState(ctx context.Context, objectInstance *operatorv1alpha1.Telemetry) error {
	status := getStatusFromTelemetry(objectInstance)

	return r.setStatusForObjectInstance(ctx, objectInstance, status.
		WithState(operatorv1alpha1.StateProcessing).
		WithInstallConditionStatus(metav1.ConditionUnknown, objectInstance.GetGeneration()))
}

// HandleProcessingState processes the reconciled resource by processing the underlying resources.
// Based on the processing either a success or failure state is set on the reconciled resource.
func (r *Reconciler) HandleProcessingState(ctx context.Context, objectInstance *operatorv1alpha1.Telemetry) error {
	status := getStatusFromTelemetry(objectInstance)

	// set eventual state to Ready - if no errors were found
	return r.setStatusForObjectInstance(ctx, objectInstance, status.
		WithState(operatorv1alpha1.StateReady).
		WithInstallConditionStatus(metav1.ConditionTrue, objectInstance.GetGeneration()))
}

func (r *Reconciler) HandleDeletingState(ctx context.Context, objectInstance *operatorv1alpha1.Telemetry) error {
	r.Event(objectInstance, "Normal", "Deleting", "resource deleting")

	err := r.deleteWebhook(ctx)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}

	// if resources are ready to be deleted, remove finalizer
	if controllerutil.RemoveFinalizer(objectInstance, finalizer) {
		return r.Client.Update(ctx, objectInstance)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.Telemetry{}).
		Complete(r)
}

func getStatusFromTelemetry(objectInstance *operatorv1alpha1.Telemetry) operatorv1alpha1.TelemetryStatus {
	return objectInstance.Status
}

func (r *Reconciler) setStatusForObjectInstance(ctx context.Context, objectInstance *operatorv1alpha1.Telemetry,
	status *operatorv1alpha1.TelemetryStatus,
) error {
	objectInstance.Status = *status

	if err := r.serverSideApplyStatus(ctx, objectInstance); err != nil {
		r.Event(objectInstance, "Warning", "ErrorUpdatingStatus", fmt.Sprintf("updating state to %v", string(status.State)))
		return fmt.Errorf("error while updating status %s to: %w", status.State, err)
	}

	r.Event(objectInstance, "Normal", "StatusUpdated", fmt.Sprintf("updating state to %v", string(status.State)))
	return nil
}

func (r *Reconciler) serverSideApplyStatus(ctx context.Context, obj client.Object) error {
	obj.SetManagedFields(nil)
	obj.SetResourceVersion("")
	return r.Status().Patch(ctx, obj, client.Apply,
		&client.SubResourcePatchOptions{PatchOptions: client.PatchOptions{FieldManager: fieldOwner}})
}

func (r *Reconciler) serverSideApply(ctx context.Context, obj client.Object) error {
	obj.SetManagedFields(nil)
	obj.SetResourceVersion("")
	return r.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner(fieldOwner))
}

func (r *Reconciler) customResourceExist(ctx context.Context) bool {
	return r.checkLogParserExist(ctx) ||
		r.checkLogPipelineExist(ctx) ||
		r.checkMetricPipelinesExist(ctx) ||
		r.checkTracePipelinesExist(ctx)
}

func (r *Reconciler) checkLogParserExist(ctx context.Context) bool {
	var parserList telemetryv1alpha1.LogParserList
	if err := r.List(ctx, &parserList); err != nil {
		//no kind found
		if _, ok := err.(*meta.NoKindMatchError); ok {
			return false
		}
		return true
	}
	return len(parserList.Items) > 0
}

func (r *Reconciler) checkLogPipelineExist(ctx context.Context) bool {
	var pipelineList telemetryv1alpha1.LogPipelineList
	if err := r.List(ctx, &pipelineList); err != nil {
		//no kind found
		if _, ok := err.(*meta.NoKindMatchError); ok {
			return false
		}
		return true
	}
	return len(pipelineList.Items) > 0
}

func (r *Reconciler) checkMetricPipelinesExist(ctx context.Context) bool {
	var metricPipelineList telemetryv1alpha1.MetricPipelineList
	if err := r.List(ctx, &metricPipelineList); err != nil {
		//no kind found
		if _, ok := err.(*meta.NoKindMatchError); ok {
			return false
		}
		return true
	}

	return len(metricPipelineList.Items) > 0
}

func (r *Reconciler) checkTracePipelinesExist(ctx context.Context) bool {
	var tracePipelineList telemetryv1alpha1.TracePipelineList
	if err := r.List(ctx, &tracePipelineList); err != nil {
		//no kind found
		if _, ok := err.(*meta.NoKindMatchError); ok {
			return false
		}
		return true
	}

	return len(tracePipelineList.Items) > 0
}
