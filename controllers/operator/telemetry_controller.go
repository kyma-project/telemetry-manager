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

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/predicate"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/resources/selfmonitor"
)

type TelemetryController struct {
	client.Client

	config     TelemetryControllerConfig
	reconciler *telemetry.Reconciler
}

type TelemetryControllerConfig struct {
	telemetry.Config

	SelfMonitorName    string
	TelemetryNamespace string
}

func NewTelemetryController(client client.Client, scheme *runtime.Scheme, config TelemetryControllerConfig) *TelemetryController {
	reconciler := telemetry.New(
		client,
		scheme,
		config.Config,
		overrides.New(client, overrides.HandlerConfig{SystemNamespace: config.TelemetryNamespace}),
		&selfmonitor.ApplierDeleter{Config: config.SelfMonitor.Config},
	)

	return &TelemetryController{
		Client:     client,
		config:     config,
		reconciler: reconciler,
	}
}

func (r *TelemetryController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.reconciler.Reconcile(ctx, req)
}

func (r *TelemetryController) SetupWithManager(mgr ctrl.Manager) error {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.Telemetry{}).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestForOwner(mgr.GetClient().Scheme(), mgr.GetRESTMapper(), &operatorv1alpha1.Telemetry{}),
			ctrlbuilder.WithPredicates(predicate.OwnedResourceChanged())).
		Watches(
			&admissionregistrationv1.ValidatingWebhookConfiguration{},
			handler.EnqueueRequestsFromMapFunc(r.mapWebhook),
			ctrlbuilder.WithPredicates(predicate.UpdateOrDelete())).
		Watches(
			&telemetryv1alpha1.LogPipeline{},
			handler.EnqueueRequestsFromMapFunc(r.mapLogPipeline),
			ctrlbuilder.WithPredicates(predicate.CreateOrUpdateOrDelete())).
		Watches(
			&telemetryv1alpha1.TracePipeline{},
			handler.EnqueueRequestsFromMapFunc(r.mapTracePipeline),
			ctrlbuilder.WithPredicates(predicate.CreateOrUpdateOrDelete())).
		Watches(
			&telemetryv1alpha1.MetricPipeline{},
			handler.EnqueueRequestsFromMapFunc(r.mapMetricPipeline),
			ctrlbuilder.WithPredicates(predicate.CreateOrUpdateOrDelete()))

	return b.Complete(r)
}

func (r *TelemetryController) mapWebhook(ctx context.Context, object client.Object) []reconcile.Request {
	webhook, ok := object.(*admissionregistrationv1.ValidatingWebhookConfiguration)
	if !ok {
		logf.FromContext(ctx).Error(nil, "Unable to cast object to ValidatingWebhookConfiguration")
		return nil
	}

	if webhook.Name != r.config.Webhook.CertConfig.WebhookName.Name {
		return nil
	}

	return r.createTelemetryRequests(ctx)
}

func (r *TelemetryController) mapLogPipeline(ctx context.Context, object client.Object) []reconcile.Request {
	logPipeline, ok := object.(*telemetryv1alpha1.LogPipeline)
	if !ok {
		logf.FromContext(ctx).Error(nil, "Unable to cast object to LogPipeline")
		return nil
	}

	if len(logPipeline.Status.Conditions) == 0 {
		return nil
	}

	return r.createTelemetryRequests(ctx)
}

func (r *TelemetryController) mapTracePipeline(ctx context.Context, object client.Object) []reconcile.Request {
	tracePipeline, ok := object.(*telemetryv1alpha1.TracePipeline)
	if !ok {
		logf.FromContext(ctx).Error(nil, "Unable to cast object to TracePipeline")
		return nil
	}

	if len(tracePipeline.Status.Conditions) == 0 {
		return nil
	}

	return r.createTelemetryRequests(ctx)
}

func (r *TelemetryController) mapMetricPipeline(ctx context.Context, object client.Object) []reconcile.Request {
	metricPipeline, ok := object.(*telemetryv1alpha1.MetricPipeline)
	if !ok {
		logf.FromContext(ctx).Error(nil, "Unable to cast object to MetricPipeline")
		return nil
	}

	if len(metricPipeline.Status.Conditions) == 0 {
		return nil
	}

	return r.createTelemetryRequests(ctx)
}

func (r *TelemetryController) createTelemetryRequests(ctx context.Context) []reconcile.Request {
	var telemetries operatorv1alpha1.TelemetryList

	err := r.List(ctx, &telemetries)
	if err != nil {
		logf.FromContext(ctx).Error(err, "Unable to list Telemetry CRs")
		return nil
	}

	var requests []reconcile.Request
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
