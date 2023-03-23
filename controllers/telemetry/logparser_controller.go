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

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logparser"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// LogParserReconciler reconciles a Logparser object
type LogParserReconciler struct {
	client.Client

	config     logparser.Config
	reconciler *logparser.Reconciler
}

func NewLogParserReconciler(client client.Client, reconciler *logparser.Reconciler, config logparser.Config) *LogParserReconciler {
	return &LogParserReconciler{
		Client:     client,
		reconciler: reconciler,
		config:     config,
	}
}

func (r *LogParserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.reconciler.Reconcile(ctx, req)
}

func (r *LogParserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&telemetryv1alpha1.LogParser{}).
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
