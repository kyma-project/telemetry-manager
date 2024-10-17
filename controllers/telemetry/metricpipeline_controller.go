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
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric/agent"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric/gateway"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/predicate"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/metricpipeline"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	"github.com/kyma-project/telemetry-manager/internal/validators/endpoint"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

const (
	maxMetricPipelines    = 3
	metricGatewayBaseName = "telemetry-metric-gateway"
	metricAgentBaseName   = "telemetry-metric-agent"
)

var (
	metricAgentCPULimit      = resource.MustParse("1")
	metricAgentMemoryLimit   = resource.MustParse("1200Mi")
	metricAgentCPURequest    = resource.MustParse("15m")
	metricAgentMemoryRequest = resource.MustParse("50Mi")

	metricGatewayBaseCPULimit         = resource.MustParse("900m")
	metricGatewayDynamicCPULimit      = resource.MustParse("100m")
	metricGatewayBaseMemoryLimit      = resource.MustParse("512Mi")
	metricGatewayDynamicMemoryLimit   = resource.MustParse("512Mi")
	metricGatewayBaseCPURequest       = resource.MustParse("25m")
	metricGatewayDynamicCPURequest    = resource.MustParse("0")
	metricGatewayBaseMemoryRequest    = resource.MustParse("32Mi")
	metricGatewayDynamicMemoryRequest = resource.MustParse("0")
)

// MetricPipelineController reconciles a MetricPipeline object
type MetricPipelineController struct {
	client.Client

	reconcileTriggerChan <-chan event.GenericEvent
	reconciler           *metricpipeline.Reconciler
}

type MetricPipelineControllerConfig struct {
	MetricAgentPriorityClassName   string
	MetricGatewayPriorityClassName string
	MetricGatewayServiceName       string
	ModuleVersion                  string
	OTelCollectorImage             string
	RestConfig                     *rest.Config
	SelfMonitorName                string
	TelemetryNamespace             string
}

func NewMetricPipelineController(client client.Client, reconcileTriggerChan <-chan event.GenericEvent, config MetricPipelineControllerConfig) (*MetricPipelineController, error) {
	flowHealthProber, err := prober.NewMetricPipelineProber(types.NamespacedName{Name: config.SelfMonitorName, Namespace: config.TelemetryNamespace})
	if err != nil {
		return nil, err
	}

	pipelineLock := resourcelock.New(
		client,
		types.NamespacedName{
			Name:      "telemetry-metricpipeline-lock",
			Namespace: config.TelemetryNamespace,
		},
		maxMetricPipelines,
	)

	pipelineValidator := &metricpipeline.Validator{
		EndpointValidator:  &endpoint.Validator{Client: client},
		TLSCertValidator:   tlscert.New(client),
		SecretRefValidator: &secretref.Validator{Client: client},
		PipelineLock:       pipelineLock,
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config.RestConfig)
	if err != nil {
		return nil, err
	}

	reconcilerConfig := metricpipeline.Config{
		AgentName:          metricAgentBaseName,
		GatewayName:        metricGatewayBaseName,
		ModuleVersion:      config.ModuleVersion,
		TelemetryNamespace: config.TelemetryNamespace,
	}
	reconciler := metricpipeline.New(
		client,
		reconcilerConfig,
		newMetricAgentApplierDeleter(config),
		&agent.Builder{
			Config: agent.BuilderConfig{
				GatewayOTLPServiceName: types.NamespacedName{Namespace: config.TelemetryNamespace, Name: config.MetricGatewayServiceName},
			},
		},
		&workloadstatus.DaemonSetProber{Client: client},
		flowHealthProber,
		newMetricGatewayApplierDeleter(config),
		&gateway.Builder{Reader: client},
		&workloadstatus.DeploymentProber{Client: client},
		istiostatus.NewChecker(discoveryClient),
		overrides.New(client, overrides.HandlerConfig{SystemNamespace: config.TelemetryNamespace}),
		pipelineLock,
		pipelineValidator,
		&conditions.ErrorToMessageConverter{},
	)

	return &MetricPipelineController{
		Client:               client,
		reconcileTriggerChan: reconcileTriggerChan,
		reconciler:           reconciler,
	}, nil
}

func newMetricAgentApplierDeleter(config MetricPipelineControllerConfig) *otelcollector.AgentApplierDeleter {
	rbac := otelcollector.MakeMetricAgentRBAC(
		types.NamespacedName{
			Name:      metricAgentBaseName,
			Namespace: config.TelemetryNamespace,
		},
	)

	agentConfig := otelcollector.AgentConfig{
		Config: otelcollector.Config{
			BaseName:  metricAgentBaseName,
			Namespace: config.TelemetryNamespace,
		},
		DaemonSet: otelcollector.DaemonSetConfig{
			Image:             config.OTelCollectorImage,
			PriorityClassName: config.MetricAgentPriorityClassName,
			CPULimit:          metricAgentCPULimit,
			MemoryLimit:       metricAgentMemoryLimit,
			CPURequest:        metricAgentCPURequest,
			MemoryRequest:     metricAgentMemoryRequest,
		},
	}

	return &otelcollector.AgentApplierDeleter{
		Config: agentConfig,
		RBAC:   rbac,
	}
}

func newMetricGatewayApplierDeleter(config MetricPipelineControllerConfig) *otelcollector.GatewayApplierDeleter {
	rbac := otelcollector.MakeMetricGatewayRBAC(
		types.NamespacedName{
			Name:      metricGatewayBaseName,
			Namespace: config.TelemetryNamespace,
		},
	)

	gatewayConfig := otelcollector.GatewayConfig{
		Config: otelcollector.Config{
			BaseName:  metricGatewayBaseName,
			Namespace: config.TelemetryNamespace,
		},
		Deployment: otelcollector.DeploymentConfig{
			Image:                config.OTelCollectorImage,
			PriorityClassName:    config.MetricGatewayPriorityClassName,
			BaseCPULimit:         metricGatewayBaseCPULimit,
			DynamicCPULimit:      metricGatewayDynamicCPULimit,
			BaseMemoryLimit:      metricGatewayBaseMemoryLimit,
			DynamicMemoryLimit:   metricGatewayDynamicMemoryLimit,
			BaseCPURequest:       metricGatewayBaseCPURequest,
			DynamicCPURequest:    metricGatewayDynamicCPURequest,
			BaseMemoryRequest:    metricGatewayBaseMemoryRequest,
			DynamicMemoryRequest: metricGatewayDynamicMemoryRequest,
		},
		OTLPServiceName: config.MetricGatewayServiceName,
	}

	return &otelcollector.GatewayApplierDeleter{
		Config: gatewayConfig,
		RBAC:   rbac,
	}
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
		&operatorv1alpha1.Telemetry{},
		handler.EnqueueRequestsFromMapFunc(r.mapTelemetryChanges),
		ctrlbuilder.WithPredicates(predicate.CreateOrUpdateOrDelete()),
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
