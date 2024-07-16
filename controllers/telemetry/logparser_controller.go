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

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/k8sutils"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/predicate"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logparser"
)

// LogParserController reconciles a Logparser object
type LogParserController struct {
	client.Client

	reconciler *logparser.Reconciler
}

type LogParserControllerConfig struct {
	OverridesConfigMapKey  string
	OverridesConfigMapName string
	TelemetryNamespace     string
}

func NewLogParserController(client client.Client, atomicLevel zap.AtomicLevel, config LogParserControllerConfig) *LogParserController {
	overridesHandler := overrides.New(client, atomicLevel, overrides.HandlerConfig{
		ConfigMapName: types.NamespacedName{Name: config.OverridesConfigMapName, Namespace: config.TelemetryNamespace},
		ConfigMapKey:  config.OverridesConfigMapKey,
	})

	reconcilerCfg := logparser.Config{
		ParsersConfigMap: types.NamespacedName{Name: "telemetry-fluent-bit-parsers", Namespace: config.TelemetryNamespace},
		DaemonSet:        types.NamespacedName{Name: "telemetry-fluent-bit", Namespace: config.TelemetryNamespace},
	}

	reconciler := logparser.New(
		client,
		reconcilerCfg,
		&k8sutils.DaemonSetProber{Client: client},
		&k8sutils.DaemonSetAnnotator{Client: client},
		overridesHandler,
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
			ctrlbuilder.WithPredicates(predicate.OwnedResourceChanged())).
		Complete(r)
}
