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
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logparser"
	"github.com/kyma-project/telemetry-manager/internal/setup"
)

// LogparserReconciler reconciles a Logparser object
type LogParserReconciler struct {
	client.Client

	reconciler *logparser.Reconciler

	config           logparser.Config
	overridesHandler *overrides.Handler
}

func NewLogParserReconciler(client client.Client, reconciler *logparser.Reconciler, config logparser.Config, handler *overrides.Handler) *LogParserReconciler {
	return &LogParserReconciler{
		Client:           client,
		reconciler:       reconciler,
		config:           config,
		overridesHandler: handler,
	}
}

func (r *LogParserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.V(1).Info("Reconciliation triggered")

	overrideConfig, err := r.overridesHandler.UpdateOverrideConfig(ctx, r.config.OverrideConfigMap)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.overridesHandler.CheckGlobalConfig(overrideConfig.Global); err != nil {
		return ctrl.Result{}, err
	}

	if overrideConfig.Logging.Paused {
		log.V(1).Info("Skipping reconciliation of logparser as reconciliation is paused.")
		return ctrl.Result{}, nil
	}

	var parser telemetryv1alpha1.LogParser
	if err := r.Get(ctx, req.NamespacedName, &parser); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return ctrl.Result{}, r.reconciler.DoReconcile(ctx, &parser)
}

func (r *LogParserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&telemetryv1alpha1.LogParser{}).
		Watches(
			&source.Kind{Type: &appsv1.DaemonSet{}},
			handler.EnqueueRequestsFromMapFunc(r.mapDaemonSets),
			builder.WithPredicates(setup.OnlyUpdate()),
		).
		Complete(r)
}

func (r *LogParserReconciler) mapDaemonSets(object client.Object) []reconcile.Request {
	daemonSet := object.(*appsv1.DaemonSet)

	var requests []reconcile.Request
	if daemonSet.Name != r.config.DaemonSet.Name || daemonSet.Namespace != r.config.DaemonSet.Namespace {
		return requests
	}

	var allParsers telemetryv1alpha1.LogParserList
	if err := r.List(context.Background(), &allParsers); err != nil {
		ctrl.Log.Error(err, "DamonSet UpdateEvent: fetching LogParserList failed!", err.Error())
		return requests
	}

	for _, parser := range allParsers.Items {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: parser.Name}})
	}
	ctrl.Log.V(1).Info(fmt.Sprintf("DaemonSet changed event handling done: Created %d new reconciliation requests.\n", len(requests)))
	return requests
}
