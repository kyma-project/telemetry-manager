/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package telemetry

import (
	"context"
	"fmt"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

const (
	requeueInterval = time.Second * 10
	finalizer       = "telemetrymanager.kyma-project.io/finalizer"
	fieldOwner      = "telemetrymanager.kyma-project.io/owner"
)

type TelemetryManagerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	*rest.Config
	// EventRecorder for creating k8s events
	record.EventRecorder
}

//+kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=telemetrymanagers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=telemetrymanagers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=telemetry.kyma-project.io,resources=telemetrymanagers/finalizers,verbs=update

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *TelemetryManagerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	objectInstance := telemetryv1alpha1.TelemetryManager{}

	if err := r.Client.Get(ctx, req.NamespacedName, &objectInstance); err != nil {
		logger.Info(req.NamespacedName.String() + " got deleted!")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// check if deletionTimestamp is set, retry until it gets deleted
	status := getStatusFromTelemetryManager(&objectInstance)

	if !objectInstance.GetDeletionTimestamp().IsZero() &&
		status.State != telemetryv1alpha1.StateDeleting {
		// if the status is not yet set to deleting, also update the status
		return ctrl.Result{}, r.setStatusForObjectInstance(ctx, &objectInstance, status.WithState(telemetryv1alpha1.StateDeleting))
	}

	// add finalizer if not present
	if controllerutil.AddFinalizer(&objectInstance, finalizer) {
		return ctrl.Result{}, r.serverSideApply(ctx, &objectInstance)
	}

	switch status.State {
	case "":
		return ctrl.Result{}, r.HandleInitialState(ctx, &objectInstance)
	case telemetryv1alpha1.StateProcessing:
		return ctrl.Result{Requeue: true}, r.HandleProcessingState(ctx, &objectInstance)
	case telemetryv1alpha1.StateDeleting:
		return ctrl.Result{Requeue: true}, r.HandleDeletingState(ctx, &objectInstance)
	case telemetryv1alpha1.StateError:
		return ctrl.Result{Requeue: true}, r.HandleErrorState(ctx, &objectInstance)
	case telemetryv1alpha1.StateReady:
		return ctrl.Result{RequeueAfter: requeueInterval}, r.HandleReadyState(ctx, &objectInstance)
	}

	return ctrl.Result{}, nil
}

// HandleReadyState checks for the consistency of reconciled resource, by verifying the underlying resources.
func (r *TelemetryManagerReconciler) HandleReadyState(ctx context.Context, objectInstance *telemetryv1alpha1.TelemetryManager) error {
	return nil
}

// HandleErrorState handles error recovery for the reconciled resource.
func (r *TelemetryManagerReconciler) HandleErrorState(ctx context.Context, objectInstance *telemetryv1alpha1.TelemetryManager) error {
	status := getStatusFromTelemetryManager(objectInstance)

	// set eventual state to Ready - if no errors were found
	return r.setStatusForObjectInstance(ctx, objectInstance, status.
		WithState(telemetryv1alpha1.StateReady).
		WithInstallConditionStatus(metav1.ConditionTrue, objectInstance.GetGeneration()))
}

// HandleInitialState bootstraps state handling for the reconciled resource.
func (r *TelemetryManagerReconciler) HandleInitialState(ctx context.Context, objectInstance *telemetryv1alpha1.TelemetryManager) error {
	status := getStatusFromTelemetryManager(objectInstance)

	return r.setStatusForObjectInstance(ctx, objectInstance, status.
		WithState(telemetryv1alpha1.StateProcessing).
		WithInstallConditionStatus(metav1.ConditionUnknown, objectInstance.GetGeneration()))
}

// HandleProcessingState processes the reconciled resource by processing the underlying resources.
// Based on the processing either a success or failure state is set on the reconciled resource.
func (r *TelemetryManagerReconciler) HandleProcessingState(ctx context.Context, objectInstance *telemetryv1alpha1.TelemetryManager) error {
	status := getStatusFromTelemetryManager(objectInstance)

	// set eventual state to Ready - if no errors were found
	return r.setStatusForObjectInstance(ctx, objectInstance, status.
		WithState(telemetryv1alpha1.StateReady).
		WithInstallConditionStatus(metav1.ConditionTrue, objectInstance.GetGeneration()))
}

func (r *TelemetryManagerReconciler) HandleDeletingState(ctx context.Context, objectInstance *telemetryv1alpha1.TelemetryManager) error {
	r.Event(objectInstance, "Normal", "Deleting", "resource deleting")

	// if resources are ready to be deleted, remove finalizer
	if controllerutil.RemoveFinalizer(objectInstance, finalizer) {
		return r.Client.Update(ctx, objectInstance)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TelemetryManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&telemetryv1alpha1.TelemetryManager{}).
		Complete(r)
}

func getStatusFromTelemetryManager(objectInstance *telemetryv1alpha1.TelemetryManager) telemetryv1alpha1.TelemetryManagerStatus {
	return objectInstance.Status
}

func (r *TelemetryManagerReconciler) setStatusForObjectInstance(ctx context.Context, objectInstance *telemetryv1alpha1.TelemetryManager,
	status *telemetryv1alpha1.TelemetryManagerStatus,
) error {
	objectInstance.Status = *status

	if err := r.serverSideApplyStatus(ctx, objectInstance); err != nil {
		r.Event(objectInstance, "Warning", "ErrorUpdatingStatus", fmt.Sprintf("updating state to %v", string(status.State)))
		return fmt.Errorf("error while updating status %s to: %w", status.State, err)
	}

	r.Event(objectInstance, "Normal", "StatusUpdated", fmt.Sprintf("updating state to %v", string(status.State)))
	return nil
}

func (r *TelemetryManagerReconciler) serverSideApplyStatus(ctx context.Context, obj client.Object) error {
	obj.SetManagedFields(nil)
	obj.SetResourceVersion("")
	return r.Status().Patch(ctx, obj, client.Apply,
		&client.SubResourcePatchOptions{PatchOptions: client.PatchOptions{FieldManager: fieldOwner}})
}

func (r *TelemetryManagerReconciler) serverSideApply(ctx context.Context, obj client.Object) error {
	obj.SetManagedFields(nil)
	obj.SetResourceVersion("")
	return r.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner(fieldOwner))
}
