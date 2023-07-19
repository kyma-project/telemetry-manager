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

package operator

import (
	"context"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/telemetry"
)

type TelemetryReconciler struct {
	client.Client
	reconciler *telemetry.Reconciler
}

func NewTelemetryReconciler(client client.Client, reconciler *telemetry.Reconciler) *TelemetryReconciler {
	return &TelemetryReconciler{
		Client:     client,
		reconciler: reconciler,
	}
}

func (r *TelemetryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.reconciler.Reconcile(ctx, req)
}

func (r *TelemetryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.Telemetry{}).
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			&handler.EnqueueRequestForOwner{
				OwnerType:    &operatorv1alpha1.Telemetry{},
				IsController: false}).
		Watches(
			&source.Kind{Type: &admissionv1.ValidatingWebhookConfiguration{}},
			&handler.EnqueueRequestForOwner{
				OwnerType:    &operatorv1alpha1.Telemetry{},
				IsController: false}).
		Complete(r)
}
