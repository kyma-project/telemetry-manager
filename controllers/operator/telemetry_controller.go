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
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	autoscalingvpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/nodesize"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	"github.com/kyma-project/telemetry-manager/internal/resources/selfmonitor"
	predicateutils "github.com/kyma-project/telemetry-manager/internal/utils/predicate"
	"github.com/kyma-project/telemetry-manager/internal/vpastatus"
	"github.com/kyma-project/telemetry-manager/internal/webhookcert"
)

type TelemetryController struct {
	client.Client

	config          TelemetryControllerConfig
	reconciler      *telemetry.Reconciler
	nodeSizeTracker *nodesize.Tracker
}

type TelemetryControllerConfig struct {
	config.Global

	SelfMonitorAlertmanagerWebhookURL string
	SelfMonitorImage                  string
	SelfMonitorPriorityClassName      string
	WebhookCert                       webhookcert.Config
}

func NewTelemetryController(config TelemetryControllerConfig, mgr ctrl.Manager, nodeSizeTracker *nodesize.Tracker) *TelemetryController {
	client := mgr.GetClient()
	scheme := mgr.GetScheme()
	restConfig := mgr.GetConfig()

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
		telemetry.WithVpaStatusChecker(vpastatus.NewChecker(restConfig)),
		telemetry.WithNodeSizeTracker(nodeSizeTracker),
	)

	return &TelemetryController{
		Client:          client,
		config:          config,
		reconciler:      reconciler,
		nodeSizeTracker: nodeSizeTracker,
	}
}

func (r *TelemetryController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.reconciler.Reconcile(ctx, req)
}

// telemetryOwnedResourceTypes returns the list of Kubernetes resource types that are always
// managed (created/updated/deleted) by the Telemetry reconciler and must be watched for changes.
func telemetryOwnedResourceTypes(vpaCRDExists bool) []client.Object {
	resources := []client.Object{
		&appsv1.Deployment{},
		&corev1.ConfigMap{},
		&corev1.Secret{},
		&corev1.Service{},
		&corev1.ServiceAccount{},
		&rbacv1.Role{},
		&rbacv1.RoleBinding{},
		&networkingv1.NetworkPolicy{},
	}

	if vpaCRDExists {
		resources = append(resources, &autoscalingvpav1.VerticalPodAutoscaler{})
	}

	return resources
}

func (r *TelemetryController) SetupWithManager(mgr ctrl.Manager) error {
	b := ctrl.NewControllerManagedBy(mgr).For(&operatorv1beta1.Telemetry{})

	ctx := context.Background()

	vpaCRDExists, err := vpastatus.NewChecker(mgr.GetConfig()).VpaCRDExists(ctx, r.Client)
	if err != nil {
		return fmt.Errorf("failed to check VPA status: %w", err)
	}

	ownedResourceTypesToWatch := telemetryOwnedResourceTypes(vpaCRDExists)

	for _, resource := range ownedResourceTypesToWatch {
		// VPA needs special handling to ensure it's recreated after deletion
		if _, isVPA := resource.(*autoscalingvpav1.VerticalPodAutoscaler); isVPA {
			b = b.Watches(
				resource,
				handler.EnqueueRequestsFromMapFunc(r.mapVPAChanges),
			)
		} else {
			b = b.Watches(
				resource,
				handler.EnqueueRequestForOwner(
					mgr.GetClient().Scheme(),
					mgr.GetRESTMapper(),
					&operatorv1beta1.Telemetry{},
				),
				ctrlbuilder.WithPredicates(predicateutils.OwnedResourceChanged()),
			)
		}
	}

	b = b.
		Watches(
			&admissionregistrationv1.ValidatingWebhookConfiguration{},
			handler.EnqueueRequestsFromMapFunc(r.mapValidatingWebhook),
		).
		Watches(
			&admissionregistrationv1.MutatingWebhookConfiguration{},
			handler.EnqueueRequestsFromMapFunc(r.mapMutatingWebhook),
		).
		Watches(
			&apiextensionsv1.CustomResourceDefinition{},
			handler.EnqueueRequestsFromMapFunc(r.mapPipelineCRD),
		).
		Watches(
			&telemetryv1beta1.LogPipeline{},
			handler.EnqueueRequestsFromMapFunc(r.mapLogPipeline),
		).
		Watches(
			&telemetryv1beta1.TracePipeline{},
			handler.EnqueueRequestsFromMapFunc(r.mapTracePipeline),
		).
		Watches(
			&telemetryv1beta1.MetricPipeline{},
			handler.EnqueueRequestsFromMapFunc(r.mapMetricPipeline),
		).
		Watches(
			&corev1.Node{},
			handler.EnqueueRequestsFromMapFunc(r.mapNodeChanges),
		)

	return b.Complete(r)
}

func (r *TelemetryController) mapValidatingWebhook(ctx context.Context, object client.Object) []reconcile.Request {
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

func (r *TelemetryController) mapMutatingWebhook(ctx context.Context, object client.Object) []reconcile.Request {
	webhook, ok := object.(*admissionregistrationv1.MutatingWebhookConfiguration)
	if !ok {
		logf.FromContext(ctx).Error(nil, "Unable to cast object to MutatingWebhookConfiguration")
		return nil
	}

	if webhook.Name != r.config.WebhookCert.MutatingWebhookName.Name {
		return nil
	}

	return r.createTelemetryRequests(ctx)
}

func (r *TelemetryController) mapPipelineCRD(ctx context.Context, object client.Object) []reconcile.Request {
	crd, ok := object.(*apiextensionsv1.CustomResourceDefinition)
	if !ok {
		logf.FromContext(ctx).Error(nil, "Unable to cast object to CustomResourceDefinition")
		return nil
	}

	// Telemetry controller only patches LogPipeline and MetricPipeline CRDs with conversion webhook configuration
	if crd.Name != names.LogPipelineCRD && crd.Name != names.MetricPipelineCRD {
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

// mapNodeChanges updates the node size tracker when a Node is added, removed, or modified.
// If the smallest node memory or node count changes, it enqueues a reconciliation request
// for all Telemetry CRs so that self-monitor VPA max memory can be recalculated.
func (r *TelemetryController) mapNodeChanges(ctx context.Context, object client.Object) []reconcile.Request {
	changed, err := r.nodeSizeTracker.UpdateSmallestMemory(ctx)
	if err != nil {
		logf.FromContext(ctx).Error(err, "Unable to update smallest node memory")
		return nil
	}

	if !changed {
		return nil
	}

	return r.createTelemetryRequests(ctx)
}

// mapVPAChanges handles VPA resource changes (create, update, delete) for self-monitor.
// This ensures the VPA is recreated if manually deleted and updated when node count changes.
func (r *TelemetryController) mapVPAChanges(ctx context.Context, object client.Object) []reconcile.Request {
	vpa, ok := object.(*autoscalingvpav1.VerticalPodAutoscaler)
	if !ok {
		logf.FromContext(ctx).Error(nil, "Unable to cast object to VerticalPodAutoscaler")
		return nil
	}

	// Only handle self-monitor VPA
	if vpa.Name != names.SelfMonitor {
		return nil
	}

	logf.FromContext(ctx).V(1).Info("Self-monitor VPA changed, triggering reconciliation", "vpa", vpa.Name)

	return r.createTelemetryRequests(ctx)
}
