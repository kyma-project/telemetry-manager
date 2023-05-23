package telemetry

import (
	"context"
	"fmt"
	"time"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/kubernetes"
	"github.com/kyma-project/telemetry-manager/internal/webhookcert"
)

const (
	requeueInterval = time.Second * 10
	finalizer       = "telemetry.kyma-project.io/finalizer"
	fieldOwner      = "telemetry.kyma-project.io/owner"
)

type WebhookConfig struct {
	Enabled bool
	Service types.NamespacedName
	CertDir string
}

type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
	*rest.Config
	// EventRecorder for creating k8s events
	record.EventRecorder
	webhookConfig WebhookConfig
}

func NewReconciler(client client.Client, scheme *runtime.Scheme, eventRecorder record.EventRecorder, webhookConfig WebhookConfig) *Reconciler {
	return &Reconciler{
		Client:        client,
		Scheme:        scheme,
		EventRecorder: eventRecorder,
		webhookConfig: webhookConfig,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	objectInstance := operatorv1alpha1.Telemetry{}

	if err := r.Client.Get(ctx, req.NamespacedName, &objectInstance); err != nil {
		logger.Info(req.NamespacedName.String() + " got deleted!")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// check if deletionTimestamp is set, retry until it gets deleted
	status := getStatusFromTelemetry(&objectInstance)

	if !objectInstance.GetDeletionTimestamp().IsZero() &&
		status.State != operatorv1alpha1.StateDeleting {
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
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) reconcileWebhook(ctx context.Context, telemetry *operatorv1alpha1.Telemetry) error {
	if !r.webhookConfig.Enabled {
		return nil
	}

	var webhook *admissionv1.ValidatingWebhookConfiguration
	var secret *corev1.Secret
	var err error

	if webhook, secret, err = webhookcert.EnsureCertificate(ctx, r.Client, r.webhookConfig.Service, r.webhookConfig.CertDir); err != nil {
		return fmt.Errorf("failed to reconcile webhook: %w", err)
	}

	if err = controllerutil.SetOwnerReference(telemetry, secret, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference for secret: %w", err)
	}
	if err = kubernetes.CreateOrUpdateSecret(ctx, r.Client, secret); err != nil {
		return fmt.Errorf("failed to update secret: %w", err)
	}

	if err = controllerutil.SetOwnerReference(telemetry, webhook, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference for webhook: %w", err)
	}
	if err = kubernetes.CreateOrUpdateValidatingWebhookConfiguration(ctx, r.Client, webhook); err != nil {
		return fmt.Errorf("failed to update webhook: %w", err)
	}

	return nil
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
