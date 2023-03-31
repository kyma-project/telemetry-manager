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
	"errors"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
	"github.com/kyma-project/telemetry-manager/internal/setup"
)

var (
	errIncorrectSecretObject    = errors.New("incorrect secret object")
	errIncorrectDaemonSetObject = errors.New("incorrect daemon set object")
)

// LogPipelineReconciler reconciles a LogPipeline object
type LogPipelineReconciler struct {
	client.Client

	reconciler *logpipeline.Reconciler

	config logpipeline.Config
}

func NewLogPipelineReconciler(client client.Client, reconciler *logpipeline.Reconciler, config logpipeline.Config) *LogPipelineReconciler {
	return &LogPipelineReconciler{
		Client:     client,
		reconciler: reconciler,
		config:     config,
	}
}

func (r *LogPipelineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.reconciler.Reconcile(ctx, req)
}

func (r *LogPipelineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&telemetryv1alpha1.LogPipeline{}).
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			handler.EnqueueRequestsFromMapFunc(r.mapSecret),
			builder.WithPredicates(setup.CreateOrUpdate()),
		).
		Watches(
			&source.Kind{Type: &appsv1.DaemonSet{}},
			handler.EnqueueRequestsFromMapFunc(r.mapDaemonSet),
			builder.WithPredicates(setup.DeleteOrUpdate()),
		).
		Complete(r)
}

func (r *LogPipelineReconciler) mapSecret(object client.Object) []reconcile.Request {
	var pipelines telemetryv1alpha1.LogPipelineList
	var requests []reconcile.Request
	err := r.List(context.Background(), &pipelines)
	if err != nil {
		ctrl.Log.Error(err, "Secret UpdateEvent: fetching LogPipelineList failed!", err.Error())
		return requests
	}

	secret, ok := object.(*corev1.Secret)
	if !ok {
		ctrl.Log.V(1).Error(errIncorrectSecretObject, fmt.Sprintf("Secret object of incompatible type: %+v", object))
		return requests
	}
	ctrl.Log.V(1).Info(fmt.Sprintf("Secret UpdateEvent: handling Secret: %s", secret.Name))
	for i := range pipelines.Items {
		var pipeline = pipelines.Items[i]
		if secretref.ReferencesSecret(secret.Name, secret.Namespace, &pipeline) {
			request := reconcile.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}}
			requests = append(requests, request)
			ctrl.Log.V(1).Info(fmt.Sprintf("Secret UpdateEvent: added reconcile request for pipeline: %s", pipeline.Name))
		}
	}
	return requests
}

func (r *LogPipelineReconciler) mapDaemonSet(object client.Object) []reconcile.Request {
	var requests []reconcile.Request
	daemonSet, ok := object.(*appsv1.DaemonSet)
	if !ok {
		ctrl.Log.V(1).Error(errIncorrectDaemonSetObject, fmt.Sprintf("DaemonSet object of incompatible type: %+v", object))
		return requests
	}

	if daemonSet.Name != r.config.DaemonSet.Name || daemonSet.Namespace != r.config.DaemonSet.Namespace {
		return requests
	}

	var allPipelines telemetryv1alpha1.LogPipelineList
	if err := r.List(context.Background(), &allPipelines); err != nil {
		ctrl.Log.Error(err, "DaemonSet UpdateEvent: fetching LogPipelineList failed!", err.Error())
		return requests
	}

	for _, pipeline := range allPipelines.Items {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
	}
	ctrl.Log.V(1).Info(fmt.Sprintf("DaemonSet changed event handling done: Created %d new reconciliation requests.\n", len(requests)))
	return requests
}
