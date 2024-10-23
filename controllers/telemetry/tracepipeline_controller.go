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
	"k8s.io/apimachinery/pkg/api/resource"
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
	"github.com/kyma-project/telemetry-manager/internal/istiostatus"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/trace/gateway"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/predicate"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	"github.com/kyma-project/telemetry-manager/internal/validators/endpoint"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

const (
	maxTracePipelines    = 3
	traceGatewayBaseName = "telemetry-trace-gateway"
)

var (
	traceGatewayBaseCPULimit         = resource.MustParse("700m")
	traceGatewayDynamicCPULimit      = resource.MustParse("500m")
	traceGatewayBaseMemoryLimit      = resource.MustParse("500Mi")
	traceGatewayDynamicMemoryLimit   = resource.MustParse("1500Mi")
	traceGatewayBaseCPURequest       = resource.MustParse("100m")
	traceGatewayDynamicCPURequest    = resource.MustParse("100m")
	traceGatewayBaseMemoryRequest    = resource.MustParse("32Mi")
	traceGatewayDynamicMemoryRequest = resource.MustParse("0")
)

// TracePipelineController reconciles a TracePipeline object
type TracePipelineController struct {
	client.Client
	reconcileTriggerChan <-chan event.GenericEvent
	reconciler           *tracepipeline.Reconciler
}

type TracePipelineControllerConfig struct {
	RestConfig                    *rest.Config
	SelfMonitorName               string
	TelemetryNamespace            string
	OTelCollectorImage            string
	TraceGatewayPriorityClassName string
	TraceGatewayServiceName       string
}

func NewTracePipelineController(client client.Client, reconcileTriggerChan <-chan event.GenericEvent, config TracePipelineControllerConfig) (*TracePipelineController, error) {
	flowHealthProber, err := prober.NewTracePipelineProber(types.NamespacedName{Name: config.SelfMonitorName, Namespace: config.TelemetryNamespace})
	if err != nil {
		return nil, err
	}

	pipelineLock := resourcelock.New(
		client,
		types.NamespacedName{
			Name:      "telemetry-tracepipeline-lock",
			Namespace: config.TelemetryNamespace,
		},
		maxTracePipelines,
	)

	pipelineValidator := &tracepipeline.Validator{
		EndpointValidator:  &endpoint.Validator{Client: client},
		TLSCertValidator:   tlscert.New(client),
		SecretRefValidator: &secretref.Validator{Client: client},
		PipelineLock:       pipelineLock,
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config.RestConfig)
	if err != nil {
		return nil, err
	}

	reconcilerConfig := tracepipeline.Config{
		TraceGatewayName:   traceGatewayBaseName,
		TelemetryNamespace: config.TelemetryNamespace,
	}
	reconciler := tracepipeline.New(
		client,
		reconcilerConfig,
		flowHealthProber,
		newTraceGatewayApplierDeleter(config),
		&gateway.Builder{Reader: client},
		&workloadstatus.DeploymentProber{Client: client},
		istiostatus.NewChecker(discoveryClient),
		overrides.New(client, overrides.HandlerConfig{SystemNamespace: config.TelemetryNamespace}),
		pipelineLock,
		pipelineValidator,
		&conditions.ErrorToMessageConverter{})

	return &TracePipelineController{
		Client:               client,
		reconcileTriggerChan: reconcileTriggerChan,
		reconciler:           reconciler,
	}, nil
}

func newTraceGatewayApplierDeleter(config TracePipelineControllerConfig) *otelcollector.GatewayApplierDeleter {
	rbac := otelcollector.MakeTraceGatewayRBAC(
		types.NamespacedName{
			Name:      traceGatewayBaseName,
			Namespace: config.TelemetryNamespace,
		})

	gatewayConfig := otelcollector.GatewayConfig{
		Config: otelcollector.Config{
			BaseName:  traceGatewayBaseName,
			Namespace: config.TelemetryNamespace,
		},
		Deployment: otelcollector.DeploymentConfig{
			Image:                config.OTelCollectorImage,
			PriorityClassName:    config.TraceGatewayPriorityClassName,
			BaseCPULimit:         traceGatewayBaseCPULimit,
			DynamicCPULimit:      traceGatewayDynamicCPULimit,
			BaseMemoryLimit:      traceGatewayBaseMemoryLimit,
			DynamicMemoryLimit:   traceGatewayDynamicMemoryLimit,
			BaseCPURequest:       traceGatewayBaseCPURequest,
			DynamicCPURequest:    traceGatewayDynamicCPURequest,
			BaseMemoryRequest:    traceGatewayBaseMemoryRequest,
			DynamicMemoryRequest: traceGatewayDynamicMemoryRequest,
		},
		OTLPServiceName: config.TraceGatewayServiceName,
	}

	return &otelcollector.GatewayApplierDeleter{
		Config: gatewayConfig,
		RBAC:   rbac,
	}
}

func (r *TracePipelineController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.reconciler.Reconcile(ctx, req)
}

func (r *TracePipelineController) SetupWithManager(mgr ctrl.Manager) error {
	b := ctrl.NewControllerManagedBy(mgr).For(&telemetryv1alpha1.TracePipeline{})

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
				&telemetryv1alpha1.TracePipeline{},
			),
			ctrlbuilder.WithPredicates(predicate.OwnedResourceChanged()),
		)
	}

	return b.Watches(
		&operatorv1alpha1.Telemetry{},
		handler.EnqueueRequestsFromMapFunc(r.mapTelemetryChanges),
		ctrlbuilder.WithPredicates(predicate.CreateOrUpdateOrDelete()),
	).Complete(r)
}

func (r *TracePipelineController) mapTelemetryChanges(ctx context.Context, object client.Object) []reconcile.Request {
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

func (r *TracePipelineController) createRequestsForAllPipelines(ctx context.Context) ([]reconcile.Request, error) {
	var pipelines telemetryv1alpha1.TracePipelineList

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
