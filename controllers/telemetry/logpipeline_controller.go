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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
	"github.com/kyma-project/telemetry-manager/internal/setup"
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
			&source.Kind{Type: &appsv1.DaemonSet{}},
			enqueueRequestForOwnerFuncs(ctrl.Log),
			builder.WithPredicates(setup.DeleteOrUpdate()),
		).
		Watches(
			&source.Kind{Type: &corev1.Service{}},
			enqueueRequestForOwnerFuncs(ctrl.Log),
			builder.WithPredicates(setup.DeleteOrUpdate()),
		).
		Watches(
			&source.Kind{Type: &corev1.ConfigMap{}},
			enqueueRequestForOwnerFuncs(ctrl.Log),
			builder.WithPredicates(setup.DeleteOrUpdate()),
		).
		Watches(
			&source.Kind{Type: &corev1.ServiceAccount{}},
			enqueueRequestForOwnerFuncs(ctrl.Log),
			builder.WithPredicates(setup.DeleteOrUpdate()),
		).
		Watches(
			&source.Kind{Type: &rbacv1.ClusterRole{}},
			enqueueRequestForOwnerFuncs(ctrl.Log),
			builder.WithPredicates(setup.DeleteOrUpdate()),
		).
		Watches(
			&source.Kind{Type: &rbacv1.ClusterRoleBinding{}},
			enqueueRequestForOwnerFuncs(ctrl.Log),
			builder.WithPredicates(setup.DeleteOrUpdate()),
		).
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			handler.EnqueueRequestsFromMapFunc(r.mapSecret),
			builder.WithPredicates(setup.CreateOrUpdate()),
		).
		Complete(r)
}

func enqueueRequestForOwnerFuncs(log logr.Logger) handler.EventHandler {
	return &handler.Funcs{
		CreateFunc: func(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
			enqueueRequestsForOwners(log, evt.Object, q)
		},
		DeleteFunc: func(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
			enqueueRequestsForOwners(log, evt.Object, q)
		},
		UpdateFunc: func(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
			oldOwners := getOwnersFromObject(e.ObjectOld)
			newOwners := getOwnersFromObject(e.ObjectNew)

			for _, newOwner := range newOwners {
				if ownerSliceContains(oldOwners, newOwner) {
					continue
				}

				req := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: "kyma-system",
						Name:      newOwner.Name,
					},
				}
				q.Add(req)
				log.V(1).Info("Enqueued reconcile request for owner", "owner", req.NamespacedName)
			}
		},
	}
}

func enqueueRequestsForOwners(log logr.Logger, obj metav1.Object, q workqueue.RateLimitingInterface) {
	owners := getOwnersFromObject(obj)

	for _, owner := range owners {
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: "kyma-system",
				Name:      owner.Name,
			},
		}
		q.Add(req)
		log.V(1).Info("Enqueued reconcile request for owner", "owner", req.NamespacedName)
	}
}

func getOwnersFromObject(obj metav1.Object) []metav1.OwnerReference {
	if obj == nil {
		return nil
	}

	return obj.GetOwnerReferences()
}

func ownerSliceContains(owners []metav1.OwnerReference, owner metav1.OwnerReference) bool {
	for _, o := range owners {
		if o.UID == owner.UID {
			return true
		}
	}
	return false
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
		ctrl.Log.V(1).Error(errIncorrectSecretObject, "Secret object of incompatible type")
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
