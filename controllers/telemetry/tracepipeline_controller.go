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

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/istiostatus"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/tracegateway"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline"
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

// TracePipelineController reconciles a TracePipeline object
type TracePipelineController struct {
	client.Client

	reconcileTriggerChan <-chan event.GenericEvent
	reconciler           *tracepipeline.Reconciler
}

type TracePipelineControllerConfig struct {
	config.Global

	RestConfig                    *rest.Config
	OTelCollectorImage            string
	TraceGatewayPriorityClassName string
}

func NewTracePipelineController(config TracePipelineControllerConfig, client client.Client, reconcileTriggerChan <-chan event.GenericEvent) (*TracePipelineController, error) {
	flowHealthProber, err := prober.NewOTelTraceGatewayProber(types.NamespacedName{Name: selfmonitor.ServiceName, Namespace: config.TargetNamespace()})
	if err != nil {
		return nil, err
	}

	pipelineLock := resourcelock.NewLocker(
		client,
		types.NamespacedName{
			Name:      "telemetry-tracepipeline-lock",
			Namespace: config.TargetNamespace(),
		},
		MaxPipelineCount,
	)

	pipelineSync := resourcelock.NewSyncer(
		client,
		types.NamespacedName{
			Name:      "telemetry-tracepipeline-sync",
			Namespace: config.TargetNamespace(),
		},
	)

	transformSpecValidator, err := ottl.NewTransformSpecValidator(ottl.SignalTypeTrace)
	if err != nil {
		return nil, err
	}

	filterSpecValidator, err := ottl.NewFilterSpecValidator(ottl.SignalTypeTrace)
	if err != nil {
		return nil, err
	}

	pipelineValidator := tracepipeline.NewValidator(
		tracepipeline.WithEndpointValidator(&endpoint.Validator{Client: client}),
		tracepipeline.WithTLSCertValidator(tlscert.New(client)),
		tracepipeline.WithSecretRefValidator(&secretref.Validator{Client: client}),
		tracepipeline.WithValidatorPipelineLock(pipelineLock),
		tracepipeline.WithTransformSpecValidator(transformSpecValidator),
		tracepipeline.WithFilterSpecValidator(filterSpecValidator),
	)

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config.RestConfig)
	if err != nil {
		return nil, err
	}

	reconciler := tracepipeline.New(
		tracepipeline.WithClient(client),
		tracepipeline.WithGlobal(config.Global),

		tracepipeline.WithGatewayApplierDeleter(otelcollector.NewTraceGatewayApplierDeleter(config.Global, config.OTelCollectorImage, config.TraceGatewayPriorityClassName)),
		tracepipeline.WithGatewayConfigBuilder(&tracegateway.Builder{Reader: client}),
		tracepipeline.WithGatewayProber(&workloadstatus.DeploymentProber{Client: client}),

		tracepipeline.WithFlowHealthProber(flowHealthProber),
		tracepipeline.WithIstioStatusChecker(istiostatus.NewChecker(discoveryClient)),
		tracepipeline.WithOverridesHandler(overrides.New(config.Global, client)),
		tracepipeline.WithErrorToMessageConverter(&conditions.ErrorToMessageConverter{}),

		tracepipeline.WithPipelineLock(pipelineLock),
		tracepipeline.WithPipelineSyncer(pipelineSync),
		tracepipeline.WithPipelineValidator(pipelineValidator),
	)

	return &TracePipelineController{
		Client:               client,
		reconcileTriggerChan: reconcileTriggerChan,
		reconciler:           reconciler,
	}, nil
}

func (r *TracePipelineController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.reconciler.Reconcile(ctx, req)
}

func (r *TracePipelineController) SetupWithManager(mgr ctrl.Manager) error {
	b := ctrl.NewControllerManagedBy(mgr).For(&telemetryv1beta1.TracePipeline{})

	b.WatchesRawSource(
		source.Channel(r.reconcileTriggerChan, &handler.EnqueueRequestForObject{}),
	)

	ownedResourceTypesToWatch := []client.Object{
		&appsv1.Deployment{},
		&corev1.ConfigMap{},
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
				&telemetryv1beta1.TracePipeline{},
			),
			ctrlbuilder.WithPredicates(predicateutils.OwnedResourceChanged()),
		)
	}

	return b.Watches(
		&operatorv1beta1.Telemetry{},
		handler.EnqueueRequestsFromMapFunc(r.mapTelemetryChanges),
		ctrlbuilder.WithPredicates(predicateutils.CreateOrUpdateOrDelete()),
	).Complete(r)
}

func (r *TracePipelineController) mapTelemetryChanges(ctx context.Context, object client.Object) []reconcile.Request {
	_, ok := object.(*operatorv1beta1.Telemetry)
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

func (r *TracePipelineController) createRequestsForAllPipelines(ctx context.Context) ([]reconcile.Request, error) {
	var pipelines telemetryv1beta1.TracePipelineList

	var requests []reconcile.Request

	err := r.List(ctx, &pipelines)
	if err != nil {
		return nil, fmt.Errorf("failed to list TracePipelines: %w", err)
	}

	for i := range pipelines.Items {
		var pipeline = pipelines.Items[i]

		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
	}

	return requests, nil
}
