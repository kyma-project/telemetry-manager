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

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/resources/selfmonitor"
	predicateutils "github.com/kyma-project/telemetry-manager/internal/utils/predicate"
	"github.com/kyma-project/telemetry-manager/internal/webhookcert"
)

type TelemetryController struct {
	client.Client

	config     TelemetryControllerConfig
	reconciler *telemetry.Reconciler
}

type TelemetryControllerConfig struct {
	config.Global

	SelfMonitorAlertmanagerWebhookURL string
	SelfMonitorImage                  string
	SelfMonitorPriorityClassName      string
	WebhookCert                       webhookcert.Config
}

func NewTelemetryController(config TelemetryControllerConfig, client client.Client, scheme *runtime.Scheme) *TelemetryController {
	reconciler := telemetry.New(
		telemetry.Config{
			Global:                            config.Global,
			SelfMonitorAlertmanagerWebhookURL: config.SelfMonitorAlertmanagerWebhookURL,
			WebhookCert:                       config.WebhookCert,
		},
		scheme,
		client,
		overrides.New(config.Global, client),
		&selfmonitor.ApplierDeleter{
			Config: selfmonitor.Config{
				Global:            config.Global,
				Image:             config.SelfMonitorImage,
				PriorityClassName: config.SelfMonitorPriorityClassName,
			},
		},
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
		For(&operatorv1beta1.Telemetry{}).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestForOwner(mgr.GetClient().Scheme(), mgr.GetRESTMapper(), &operatorv1beta1.Telemetry{}),
			ctrlbuilder.WithPredicates(predicateutils.OwnedResourceChanged())).
		Watches(
			&admissionregistrationv1.ValidatingWebhookConfiguration{},
			handler.EnqueueRequestsFromMapFunc(r.mapWebhook),
			ctrlbuilder.WithPredicates(predicateutils.UpdateOrDelete())).
		Watches(
			&telemetryv1beta1.LogPipeline{},
			handler.EnqueueRequestsFromMapFunc(r.mapLogPipeline),
			ctrlbuilder.WithPredicates(predicateutils.CreateOrUpdateOrDelete())).
		Watches(
			&telemetryv1beta1.TracePipeline{},
			handler.EnqueueRequestsFromMapFunc(r.mapTracePipeline),
			ctrlbuilder.WithPredicates(predicateutils.CreateOrUpdateOrDelete())).
		Watches(
			&telemetryv1beta1.MetricPipeline{},
			handler.EnqueueRequestsFromMapFunc(r.mapMetricPipeline),
			ctrlbuilder.WithPredicates(predicateutils.CreateOrUpdateOrDelete()))

	return b.Complete(r)
}

func (r *TelemetryController) mapWebhook(ctx context.Context, object client.Object) []reconcile.Request {
	webhook, ok := object.(*admissionregistrationv1.ValidatingWebhookConfiguration)
	if !ok {
		logf.FromContext(ctx).Error(nil, "Unable to cast object to ValidatingWebhookConfiguration")
		return nil
	}

	if webhook.Name != r.config.WebhookCert.ValidatingWebhookName.Name {
		return nil
	}

	return r.createTelemetryRequests(ctx)
}

func (r *TelemetryController) mapLogPipeline(ctx context.Context, object client.Object) []reconcile.Request {
	logPipeline, ok := object.(*telemetryv1beta1.LogPipeline)
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
	tracePipeline, ok := object.(*telemetryv1beta1.TracePipeline)
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
	metricPipeline, ok := object.(*telemetryv1beta1.MetricPipeline)
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
	var telemetries operatorv1beta1.TelemetryList

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
