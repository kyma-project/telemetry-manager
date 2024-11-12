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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logparser"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
	predicateutils "github.com/kyma-project/telemetry-manager/internal/utils/predicate"
	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

// LogParserController reconciles a Logparser object
type LogParserController struct {
	client.Client

	reconciler *logparser.Reconciler
}

type LogParserControllerConfig struct {
	TelemetryNamespace string
}

func NewLogParserController(client client.Client, config LogParserControllerConfig) *LogParserController {
	reconcilerCfg := logparser.Config{
		ParsersConfigMap: types.NamespacedName{Name: "telemetry-fluent-bit-parsers", Namespace: config.TelemetryNamespace},
		DaemonSet:        types.NamespacedName{Name: "telemetry-fluent-bit", Namespace: config.TelemetryNamespace},
	}

	reconciler := logparser.New(
		client,
		reconcilerCfg,
		&workloadstatus.DaemonSetProber{Client: client},
		&k8sutils.DaemonSetAnnotator{Client: client},
		overrides.New(client, overrides.HandlerConfig{SystemNamespace: config.TelemetryNamespace}),
		&conditions.ErrorToMessageConverter{},
	)

	return &LogParserController{
		Client:     client,
		reconciler: reconciler,
	}
}

func (r *LogParserController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.reconciler.Reconcile(ctx, req)
}

func (r *LogParserController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&telemetryv1alpha1.LogParser{}).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestForOwner(mgr.GetClient().Scheme(), mgr.GetRESTMapper(), &telemetryv1alpha1.LogParser{}),
			ctrlbuilder.WithPredicates(predicateutils.OwnedResourceChanged())).
		Complete(r)
}
