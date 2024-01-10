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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/metricpipeline"
	"github.com/kyma-project/telemetry-manager/internal/setup"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// MetricPipelineReconciler reconciles a MetricPipeline object
type MetricPipelineReconciler struct {
	client.Client

	reconciler  *metricpipeline.Reconciler
	alertEvents chan event.GenericEvent
}

func NewMetricPipelineReconciler(client client.Client, alertEvents chan event.GenericEvent, reconciler *metricpipeline.Reconciler) *MetricPipelineReconciler {
	return &MetricPipelineReconciler{
		Client:      client,
		reconciler:  reconciler,
		alertEvents: alertEvents,
	}
}

func (r *MetricPipelineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.reconciler.Reconcile(ctx, req)
}

// SetupWithManager sets up the controller with the Manager.
func (r *MetricPipelineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// We use `Watches` instead of `Owns` to trigger a reconciliation also when owned objects without the controller flag are changed.
	return ctrl.NewControllerManagedBy(mgr).
		For(&telemetryv1alpha1.MetricPipeline{}).
		WatchesRawSource(&source.Channel{Source: r.alertEvents},
			handler.EnqueueRequestsFromMapFunc(r.mapPrometheusAlertEvent)).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestForOwner(mgr.GetClient().Scheme(), mgr.GetRESTMapper(), &telemetryv1alpha1.MetricPipeline{})).
		Watches(
			&appsv1.Deployment{},
			handler.EnqueueRequestForOwner(mgr.GetClient().Scheme(), mgr.GetRESTMapper(), &telemetryv1alpha1.MetricPipeline{})).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestForOwner(mgr.GetClient().Scheme(), mgr.GetRESTMapper(), &telemetryv1alpha1.MetricPipeline{})).
		Watches(
			&corev1.Service{},
			handler.EnqueueRequestForOwner(mgr.GetClient().Scheme(), mgr.GetRESTMapper(), &telemetryv1alpha1.MetricPipeline{})).
		Watches(
			&networkingv1.NetworkPolicy{},
			handler.EnqueueRequestForOwner(mgr.GetClient().Scheme(), mgr.GetRESTMapper(), &telemetryv1alpha1.MetricPipeline{})).
		Watches(
			&apiextensionsv1.CustomResourceDefinition{},
			handler.EnqueueRequestsFromMapFunc(r.mapCRDChanges),
			builder.WithPredicates(setup.CreateOrDelete()),
		).
		Watches(
			&operatorv1alpha1.Telemetry{},
			handler.EnqueueRequestsFromMapFunc(r.mapTelemetryChanges),
			builder.WithPredicates(setup.CreateOrUpdateOrDelete()),
		).Complete(r)
}

func (r *MetricPipelineReconciler) mapPrometheusAlertEvent(ctx context.Context, _ client.Object) []reconcile.Request {
	logf.FromContext(ctx).Info("Handling Prometheus alert event")
	requests, err := r.createRequestsForAllPipelines(ctx)
	if err != nil {
		logf.FromContext(ctx).Error(err, "Unable to create reconcile requests")
	}
	return requests
}

func (r *MetricPipelineReconciler) mapCRDChanges(ctx context.Context, object client.Object) []reconcile.Request {
	_, ok := object.(*apiextensionsv1.CustomResourceDefinition)
	if !ok {
		logf.FromContext(ctx).V(1).Error(nil, "Unexpected type: expected CRD")
		return nil
	}

	requests, err := r.createRequestsForAllPipelines(ctx)
	if err != nil {
		logf.FromContext(ctx).Error(err, "Unable to create reconcile requests")
	}
	return requests
}

func (r *MetricPipelineReconciler) mapTelemetryChanges(ctx context.Context, object client.Object) []reconcile.Request {
	_, ok := object.(*operatorv1alpha1.Telemetry)
	if !ok {
		logf.FromContext(ctx).V(1).Error(nil, "Unexpected type: expected Telemetry")
		return nil
	}

	requests, err := r.createRequestsForAllPipelines(ctx)
	if err != nil {
		logf.FromContext(ctx).Error(err, "Unable to create reconcile requests")
	}
	return requests
}

func (r *MetricPipelineReconciler) createRequestsForAllPipelines(ctx context.Context) ([]reconcile.Request, error) {
	var pipelines telemetryv1alpha1.MetricPipelineList
	var requests []reconcile.Request
	err := r.List(ctx, &pipelines)
	if err != nil {
		return nil, fmt.Errorf("failed to list MetricPipelines: %w", err)
	}

	for i := range pipelines.Items {
		var pipeline = pipelines.Items[i]
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
	}

	return requests, nil
}
