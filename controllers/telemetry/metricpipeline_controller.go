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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
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
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/istiostatus"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metricagent"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metricgateway"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/metricpipeline"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/resources/selfmonitor"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	predicateutils "github.com/kyma-project/telemetry-manager/internal/utils/predicate"
	"github.com/kyma-project/telemetry-manager/internal/validators/endpoint"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

// MetricPipelineController reconciles a MetricPipeline object
type MetricPipelineController struct {
	client.Client

	reconcileTriggerChan <-chan event.GenericEvent
	reconciler           *metricpipeline.Reconciler
}

type MetricPipelineControllerConfig struct {
	config.Global

	MetricAgentPriorityClassName   string
	MetricGatewayPriorityClassName string
	OTelCollectorImage             string
	RestConfig                     *rest.Config
}

func NewMetricPipelineController(config MetricPipelineControllerConfig, client client.Client, reconcileTriggerChan <-chan event.GenericEvent) (*MetricPipelineController, error) {
	gatewayFlowHealthProber, err := prober.NewOTelMetricGatewayProber(types.NamespacedName{Name: selfmonitor.ServiceName, Namespace: config.TargetNamespace()})
	if err != nil {
		return nil, err
	}

	agentFlowHealthProber, err := prober.NewOTelMetricAgentProber(types.NamespacedName{Name: selfmonitor.ServiceName, Namespace: config.TargetNamespace()})
	if err != nil {
		return nil, err
	}

	pipelineLock := resourcelock.NewLocker(
		client,
		types.NamespacedName{
			Name:      "telemetry-metricpipeline-lock",
			Namespace: config.TargetNamespace(),
		},
		MaxPipelineCount,
	)

	pipelineSync := resourcelock.NewSyncer(
		client,
		types.NamespacedName{
			Name:      "telemetry-metricpipeline-sync",
			Namespace: config.TargetNamespace(),
		},
	)

	transformSpecValidator, err := ottl.NewTransformSpecValidator(ottl.SignalTypeMetric)
	if err != nil {
		return nil, err
	}

	filterSpecValidator, err := ottl.NewFilterSpecValidator(ottl.SignalTypeMetric)
	if err != nil {
		return nil, err
	}

	pipelineValidator := metricpipeline.NewValidator(
		metricpipeline.WithEndpointValidator(&endpoint.Validator{Client: client}),
		metricpipeline.WithTLSCertValidator(tlscert.New(client)),
		metricpipeline.WithSecretRefValidator(&secretref.Validator{Client: client}),
		metricpipeline.WithValidatorPipelineLock(pipelineLock),
		metricpipeline.WithTransformSpecValidator(transformSpecValidator),
		metricpipeline.WithFilterSpecValidator(filterSpecValidator),
	)

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config.RestConfig)
	if err != nil {
		return nil, err
	}

	agentConfigBuilder := &metricagent.Builder{Reader: client}
	gatewayConfigBuilder := &metricgateway.Builder{Reader: client}

	reconciler := metricpipeline.New(
		metricpipeline.WithClient(client),
		metricpipeline.WithGlobals(config.Global),

		metricpipeline.WithAgentApplierDeleter(otelcollector.NewMetricAgentApplierDeleter(config.Global, config.OTelCollectorImage, config.MetricAgentPriorityClassName)),
		metricpipeline.WithAgentConfigBuilder(agentConfigBuilder),
		metricpipeline.WithAgentFlowHealthProber(agentFlowHealthProber),
		metricpipeline.WithAgentProber(&workloadstatus.DaemonSetProber{Client: client}),

		metricpipeline.WithGatewayApplierDeleter(otelcollector.NewMetricGatewayApplierDeleter(config.Global, config.OTelCollectorImage, config.MetricGatewayPriorityClassName)),
		metricpipeline.WithGatewayConfigBuilder(gatewayConfigBuilder),
		metricpipeline.WithGatewayFlowHealthProber(gatewayFlowHealthProber),
		metricpipeline.WithGatewayProber(&workloadstatus.DeploymentProber{Client: client}),

		metricpipeline.WithErrorToMessageConverter(&conditions.ErrorToMessageConverter{}),
		metricpipeline.WithIstioStatusChecker(istiostatus.NewChecker(discoveryClient)),
		metricpipeline.WithOverridesHandler(overrides.New(config.Global, client)),
		metricpipeline.WithPipelineValidator(pipelineValidator),

		metricpipeline.WithPipelineLock(pipelineLock),
		metricpipeline.WithPipelineSyncer(pipelineSync),
	)

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
			ctrlbuilder.WithPredicates(predicateutils.OwnedResourceChanged()),
		)
	}

	return b.Watches(
		&operatorv1alpha1.Telemetry{},
		handler.EnqueueRequestsFromMapFunc(r.mapTelemetryChanges),
		ctrlbuilder.WithPredicates(predicateutils.CreateOrUpdateOrDelete()),
	).Complete(r)
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
