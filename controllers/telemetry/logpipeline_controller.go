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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/predicate"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline"
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
	b := ctrl.NewControllerManagedBy(mgr).For(&telemetryv1alpha1.LogPipeline{})

	ownedResourceTypesToWatch := []client.Object{
		&appsv1.DaemonSet{},
		&corev1.ConfigMap{},
		&corev1.Secret{},
		&corev1.Service{},
		&corev1.ServiceAccount{},
		&rbacv1.ClusterRole{},
		&rbacv1.ClusterRoleBinding{},
	}

	for _, resource := range ownedResourceTypesToWatch {
		b = b.Watches(
			resource,
			handler.EnqueueRequestForOwner(mgr.GetClient().Scheme(),
				mgr.GetRESTMapper(),
				&telemetryv1alpha1.LogPipeline{},
			),
			builder.WithPredicates(predicate.OwnedResourceChanged()),
		)
	}

	return b.Complete(r)
}
