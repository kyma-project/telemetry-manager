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
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/istiostatus"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric/agent"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric/gateway"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/predicate"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/metricpipeline"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	"github.com/kyma-project/telemetry-manager/internal/tlscert"
	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

// MetricPipelineController reconciles a MetricPipeline object
type MetricPipelineController struct {
	client.Client
	reconcileTriggerChan <-chan event.GenericEvent
	reconciler           *metricpipeline.Reconciler
}

type MetricPipelineControllerConfig struct {
	metricpipeline.Config

	SelfMonitorName    string
	TelemetryNamespace string
}

func NewMetricPipelineController(client client.Client, reconcileTriggerChan <-chan event.GenericEvent, config MetricPipelineControllerConfig) (*MetricPipelineController, error) {
	flowHealthProber, err := prober.NewMetricPipelineProber(types.NamespacedName{Name: config.SelfMonitorName, Namespace: config.TelemetryNamespace})
	if err != nil {
		return nil, err
	}

	pipelineLock := resourcelock.New(client, types.NamespacedName{Name: "telemetry-metricpipeline-lock", Namespace: config.Gateway.Namespace}, config.MaxPipelines)

	pipelineValidator := &metricpipeline.Validator{
		TLSCertValidator:   tlscert.New(client),
		SecretRefValidator: &secretref.Validator{Client: client},
		PipelineLock:       pipelineLock,
	}

	reconciler := metricpipeline.New(
		client,
		config.Config,
		&otelcollector.AgentApplierDeleter{Config: config.Agent},
		&agent.Builder{
			Config: agent.BuilderConfig{
				GatewayOTLPServiceName: types.NamespacedName{Namespace: config.TelemetryNamespace, Name: config.Gateway.OTLPServiceName},
			},
		},
		&workloadstatus.DaemonSetProber{Client: client},
		flowHealthProber,
		&otelcollector.GatewayApplierDeleter{Config: config.Gateway},
		&gateway.Builder{Reader: client},
		&workloadstatus.DeploymentProber{Client: client},
		istiostatus.NewChecker(client),
		overrides.New(client, overrides.HandlerConfig{SystemNamespace: config.TelemetryNamespace}),
		pipelineLock,
		pipelineValidator)

	return &MetricPipelineController{
		Client:               client,
		reconcileTriggerChan: reconcileTriggerChan,
		reconciler:           reconciler,
	}, nil
}

func (r *MetricPipelineController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.reconciler.Reconcile(ctx, req)
}

// SetupWithManager sets up the controller with the Manager.
func (r *MetricPipelineController) SetupWithManager(mgr ctrl.Manager) error {
	b := ctrl.NewControllerManagedBy(mgr).For(&telemetryv1alpha1.MetricPipeline{})

	b.WatchesRawSource(
		source.Channel(r.reconcileTriggerChan, &handler.EnqueueRequestForObject{}),
	)

	ownedResourceTypesToWatch := []client.Object{
		&appsv1.Deployment{},
		&appsv1.DaemonSet{},
		&corev1.ConfigMap{},
		&corev1.Pod{},
		&corev1.Secret{},
		&corev1.Service{},
		&corev1.ServiceAccount{},
		&rbacv1.ClusterRole{},
		&rbacv1.ClusterRoleBinding{},
		&networkingv1.NetworkPolicy{},
	}

	for _, resource := range ownedResourceTypesToWatch {
		b = b.Watches(
			resource,
			handler.EnqueueRequestForOwner(mgr.GetClient().Scheme(),
				mgr.GetRESTMapper(),
				&telemetryv1alpha1.MetricPipeline{},
			),
			ctrlbuilder.WithPredicates(predicate.OwnedResourceChanged()),
		)
	}

	return b.Watches(
		&apiextensionsv1.CustomResourceDefinition{},
		handler.EnqueueRequestsFromMapFunc(r.mapCRDChanges),
		ctrlbuilder.WithPredicates(predicate.CreateOrDelete()),
	).Watches(
		&operatorv1alpha1.Telemetry{},
		handler.EnqueueRequestsFromMapFunc(r.mapTelemetryChanges),
		ctrlbuilder.WithPredicates(predicate.CreateOrUpdateOrDelete()),
	).Complete(r)
}

func (r *MetricPipelineController) mapCRDChanges(ctx context.Context, object client.Object) []reconcile.Request {
	_, ok := object.(*apiextensionsv1.CustomResourceDefinition)
	if !ok {
		logf.FromContext(ctx).V(1).Error(nil, "Unexpected type: expected CRD")
		return nil
	}

	requests, err := r.createRequestsForAllPipelines(ctx)
	if err != nil {
		logf.FromContext(ctx).Error(err, "Unable to create reconcile requests")
	}
	return requests
}

func (r *MetricPipelineController) mapTelemetryChanges(ctx context.Context, object client.Object) []reconcile.Request {
	_, ok := object.(*operatorv1alpha1.Telemetry)
	if !ok {
		logf.FromContext(ctx).V(1).Error(nil, "Unexpected type: expected Telemetry")
		return nil
	}

	requests, err := r.createRequestsForAllPipelines(ctx)
	if err != nil {
		logf.FromContext(ctx).Error(err, "Unable to create reconcile requests")
	}
	return requests
}

func (r *MetricPipelineController) createRequestsForAllPipelines(ctx context.Context) ([]reconcile.Request, error) {
	var pipelines telemetryv1alpha1.MetricPipelineList
	var requests []reconcile.Request
	err := r.List(ctx, &pipelines)
	if err != nil {
		return nil, fmt.Errorf("failed to list MetricPipelines: %w", err)
	}

	for i := range pipelines.Items {
		var pipeline = pipelines.Items[i]
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
	}

	return requests, nil
}
