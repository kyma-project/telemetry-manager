package telemetry

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

import (
	"context"
	"fmt"
	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
	"github.com/kyma-project/telemetry-manager/internal/setup"
)

// TracePipelineReconciler reconciles a TracePipeline object
type TracePipelineReconciler struct {
	client.Client

	reconciler *tracepipeline.Reconciler
}

func NewTracePipelineReconciler(client client.Client, reconciler *tracepipeline.Reconciler) *TracePipelineReconciler {
	return &TracePipelineReconciler{
		Client:     client,
		reconciler: reconciler,
	}
}

func (r *TracePipelineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.reconciler.Reconcile(ctx, req)
}

func (r *TracePipelineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// We use `Watches` instead of `Owns` to trigger a reconciliation also when owned objects without the controller flag are changed.
	return ctrl.NewControllerManagedBy(mgr).
		For(&telemetryv1alpha1.TracePipeline{}).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestForOwner(mgr.GetClient().Scheme(), mgr.GetRESTMapper(), &telemetryv1alpha1.TracePipeline{})).
		Watches(
			&appsv1.Deployment{},
			handler.EnqueueRequestForOwner(mgr.GetClient().Scheme(), mgr.GetRESTMapper(), &telemetryv1alpha1.TracePipeline{})).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestForOwner(mgr.GetClient().Scheme(), mgr.GetRESTMapper(), &telemetryv1alpha1.TracePipeline{})).
		Watches(
			&corev1.Service{},
			handler.EnqueueRequestForOwner(mgr.GetClient().Scheme(), mgr.GetRESTMapper(), &telemetryv1alpha1.TracePipeline{})).
		Watches(
			&networkingv1.NetworkPolicy{},
			handler.EnqueueRequestForOwner(mgr.GetClient().Scheme(), mgr.GetRESTMapper(), &telemetryv1alpha1.TracePipeline{})).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.mapSecret),
			builder.WithPredicates(setup.CreateOrUpdateOrDelete()),
		).
		Watches(
			&operatorv1alpha1.Telemetry{},
			handler.EnqueueRequestsFromMapFunc(r.mapTelemetryChanges),
			builder.WithPredicates(setup.CreateOrUpdateOrDelete()),
		).Complete(r)
}

func (r *TracePipelineReconciler) mapSecret(ctx context.Context, object client.Object) []reconcile.Request {
	var pipelines telemetryv1alpha1.TracePipelineList
	var requests []reconcile.Request
	err := r.List(ctx, &pipelines)
	if err != nil {
		logf.FromContext(ctx).Error(err, "Secret UpdateEvent: fetching TracePipelineList failed!", err.Error())
		return requests
	}

	secret, ok := object.(*corev1.Secret)
	if !ok {
		logf.FromContext(ctx).V(1).Error(errIncorrectSecretObject, "Secret object of incompatible type")
		return requests
	}
	logf.FromContext(ctx).V(1).Info(fmt.Sprintf("Secret UpdateEvent: handling Secret: %s", secret.Name))
	for i := range pipelines.Items {
		var pipeline = pipelines.Items[i]
		if secretref.ReferencesSecret(secret.Name, secret.Namespace, &pipeline) {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
			logf.FromContext(ctx).V(1).Info(fmt.Sprintf("Secret UpdateEvent: added reconcile request for pipeline: %s", pipeline.Name))
		}
	}
	return requests
}

func (r *TracePipelineReconciler) mapTelemetryChanges(ctx context.Context, object client.Object) []reconcile.Request {
	var pipelines telemetryv1alpha1.TracePipelineList
	var requests []reconcile.Request
	err := r.List(ctx, &pipelines)
	if err != nil {
		logf.FromContext(ctx).Error(err, "Telemetry UpdateEvent: fetching TracePipelineList failed!", err.Error())
		return requests
	}

	telemetry, ok := object.(*operatorv1alpha1.Telemetry)
	if !ok {
		logf.FromContext(ctx).V(1).Error(errIncorrectCRDObject, "Telemetry object of incompatible type")
		return requests
	}
	logf.FromContext(ctx).V(1).Info(fmt.Sprintf("Telemetry UpdateEvent: handling Telemetry: %s", telemetry.Name))
	for i := range pipelines.Items {
		var pipeline = pipelines.Items[i]

		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		logf.FromContext(ctx).V(1).Info(fmt.Sprintf("Telemetry UpdateEvent: added reconcile request for pipeline: %s", pipeline.Name))

	}
	return requests
}
