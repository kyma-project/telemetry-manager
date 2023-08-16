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
	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/setup"
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
			handler.EnqueueRequestsFromMapFunc(r.mapWebhook),
			builder.WithPredicates(setup.DeleteOrUpdate())).
		Watches(
			&source.Kind{Type: &v1alpha1.LogPipeline{}},
			handler.EnqueueRequestsFromMapFunc(r.mapTelemetryResource),
			builder.WithPredicates(setup.CreateOrUpdateOrDelete())).
		Watches(
			&source.Kind{Type: &v1alpha1.TracePipeline{}},
			handler.EnqueueRequestsFromMapFunc(r.mapTelemetryResource),
			builder.WithPredicates(setup.CreateOrUpdateOrDelete())).
		Complete(r)
}

func (r *TelemetryReconciler) mapWebhook(object client.Object) []reconcile.Request {
	var telemetries operatorv1alpha1.TelemetryList
	var requests []reconcile.Request

	webhook, ok := object.(*admissionv1.ValidatingWebhookConfiguration)
	if !ok {
		ctrl.Log.Error(nil, "unable to cast object to ValidatingWebhookConfiguration")
		return requests
	}
	if webhook.Name != r.reconciler.TelemetryConfig.Webhook.CertConfig.WebhookName.Name {
		return requests
	}

	err := r.List(context.Background(), &telemetries)
	if err != nil {
		ctrl.Log.Error(err, "unable to list Telemetry CRs")
		return requests
	}

	for _, t := range telemetries.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      t.Name,
				Namespace: t.Namespace,
			},
		})
	}

	return requests
}

func (r *TelemetryReconciler) mapTelemetryResource(object client.Object) []reconcile.Request {
	var telemetries operatorv1alpha1.TelemetryList
	var requests []reconcile.Request

	if !telemetrySubResource(object) {
		return requests
	}

	err := r.List(context.Background(), &telemetries)
	if err != nil {
		ctrl.Log.Error(err, "unable to list Telemetry CRs")
		return requests
	}

	for _, t := range telemetries.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      t.Name,
				Namespace: t.Namespace,
			},
		})
	}

	return requests
}

func telemetrySubResource(object client.Object) bool {
	var ok bool
	lp, ok := object.(*v1alpha1.LogPipeline)
	if ok && len(lp.Status.Conditions) == 0 {
		return true
	}

	tp, ok := object.(*v1alpha1.TracePipeline)
	if ok && len(tp.Status.Conditions) == 0 {
		return true
	}

	ctrl.Log.Error(nil, "unable to cast object to log pipeline, trace pipeline or metric pipeline")

	return false

}
