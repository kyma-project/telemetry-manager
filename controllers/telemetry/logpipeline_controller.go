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
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	"github.com/kyma-project/telemetry-manager/internal/istiostatus"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/log/agent"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/log/gateway"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline"
	logpipelinefluentbit "github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/fluentbit"
	logpipelineotel "github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/otel"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	predicateutils "github.com/kyma-project/telemetry-manager/internal/utils/predicate"
	"github.com/kyma-project/telemetry-manager/internal/validators/endpoint"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

// LogPipelineController reconciles a LogPipeline object
type LogPipelineController struct {
	client.Client

	reconcileTriggerChan <-chan event.GenericEvent
	reconciler           *logpipeline.Reconciler
}

type LogPipelineControllerConfig struct {
	ExporterImage               string
	FluentBitImage              string
	OTelCollectorImage          string
	FluentBitPriorityClassName  string
	LogGatewayPriorityClassName string
	LogAgentPriorityClassName   string
	RestConfig                  *rest.Config
	SelfMonitorName             string
	TelemetryNamespace          string
	ModuleVersion               string
}

func NewLogPipelineController(client client.Client, reconcileTriggerChan <-chan event.GenericEvent, config LogPipelineControllerConfig) (*LogPipelineController, error) {
	pipelineLock := resourcelock.New(
		client,
		types.NamespacedName{
			Name:      "telemetry-logpipeline-lock",
			Namespace: config.TelemetryNamespace,
		},
		MaxPipelineCount,
	)

	flowHealthProber, err := prober.NewLogPipelineProber(types.NamespacedName{Name: config.SelfMonitorName, Namespace: config.TelemetryNamespace})
	if err != nil {
		return nil, err
	}

	otelFlowHealthProber, err := prober.NewOtelLogPipelineProber(types.NamespacedName{Name: config.SelfMonitorName, Namespace: config.TelemetryNamespace})
	if err != nil {
		return nil, err
	}

	fbReconciler, err := configureFluentBitReconciler(client, config, flowHealthProber, pipelineLock)
	if err != nil {
		return nil, err
	}

	otelReconciler, err := configureOtelReconciler(client, config, otelFlowHealthProber, pipelineLock)
	if err != nil {
		return nil, err
	}

	reconciler := logpipeline.New(
		client,
		overrides.New(client, overrides.HandlerConfig{SystemNamespace: config.TelemetryNamespace}),
		fbReconciler,
		otelReconciler,
	)

	return &LogPipelineController{
		Client:               client,
		reconcileTriggerChan: reconcileTriggerChan,
		reconciler:           reconciler,
	}, nil
}

func (r *LogPipelineController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.reconciler.Reconcile(ctx, req)
}

func (r *LogPipelineController) SetupWithManager(mgr ctrl.Manager) error {
	b := ctrl.NewControllerManagedBy(mgr).For(&telemetryv1alpha1.LogPipeline{})

	b.WatchesRawSource(
		source.Channel(r.reconcileTriggerChan, &handler.EnqueueRequestForObject{}),
	)

	ownedResourceTypesToWatch := []client.Object{
		&appsv1.DaemonSet{},
		&corev1.ConfigMap{},
		&corev1.Pod{},
		&corev1.Secret{},
		&corev1.Service{},
		&corev1.ServiceAccount{},
		&rbacv1.ClusterRole{},
		&rbacv1.ClusterRoleBinding{},
	}

	for _, resource := range ownedResourceTypesToWatch {
		b = b.Watches(
			resource,
			handler.EnqueueRequestForOwner(mgr.GetClient().Scheme(),
				mgr.GetRESTMapper(),
				&telemetryv1alpha1.LogPipeline{},
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

func (r *LogPipelineController) mapTelemetryChanges(ctx context.Context, object client.Object) []reconcile.Request {
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

func configureFluentBitReconciler(client client.Client, config LogPipelineControllerConfig, flowHealthProber *prober.LogPipelineProber, pipelineLock logpipelinefluentbit.PipelineLock) (*logpipelinefluentbit.Reconciler, error) {
	pipelineValidator := &logpipelinefluentbit.Validator{
		EndpointValidator:  &endpoint.Validator{Client: client},
		TLSCertValidator:   tlscert.New(client),
		SecretRefValidator: &secretref.Validator{Client: client},
		PipelineLock:       pipelineLock,
	}

	fluentBitApplierDeleter := fluentbit.NewFluentBitApplierDeleter(
		config.TelemetryNamespace,
		config.FluentBitImage,
		config.ExporterImage,
		config.FluentBitPriorityClassName,
	)

	fluentBitConfigBuilder := builder.NewFluentBitConfigBuilder(client)

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config.RestConfig)
	if err != nil {
		return nil, err
	}

	fbReconciler := logpipelinefluentbit.New(
		client,
		config.TelemetryNamespace,
		fluentBitConfigBuilder,
		fluentBitApplierDeleter,
		&workloadstatus.DaemonSetProber{Client: client},
		flowHealthProber,
		istiostatus.NewChecker(discoveryClient),
		pipelineLock,
		pipelineValidator,
		&conditions.ErrorToMessageConverter{})

	return fbReconciler, nil
}

type FlowHealthProber interface {
	Probe(ctx context.Context, pipelineName string) (prober.OTelPipelineProbeResult, error)
}

//nolint:unparam // error is always nil: An error could be returned after implementing the IstioStatusChecker (TODO)
func configureOtelReconciler(client client.Client, config LogPipelineControllerConfig, flowHealthProber FlowHealthProber, pipelineLock logpipelineotel.PipelineLock) (*logpipelineotel.Reconciler, error) {
	pipelineValidator := &logpipelineotel.Validator{
		PipelineLock: pipelineLock,
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config.RestConfig)
	if err != nil {
		return nil, err
	}

	agentConfigBuilder := &agent.Builder{
		Reader: client,
		Config: agent.BuilderConfig{
			GatewayOTLPServiceName: types.NamespacedName{Namespace: config.TelemetryNamespace, Name: otelcollector.LogOTLPServiceName},
		},
	}

	otelReconciler := logpipelineotel.New(
		client,
		config.TelemetryNamespace,
		config.ModuleVersion,
		flowHealthProber,
		agentConfigBuilder,
		otelcollector.NewLogAgentApplierDeleter(config.OTelCollectorImage, config.TelemetryNamespace, config.LogAgentPriorityClassName),
		&workloadstatus.DaemonSetProber{Client: client},
		otelcollector.NewLogGatewayApplierDeleter(config.OTelCollectorImage, config.TelemetryNamespace, config.LogGatewayPriorityClassName),
		&gateway.Builder{Reader: client},
		&workloadstatus.DeploymentProber{Client: client},
		istiostatus.NewChecker(discoveryClient),
		pipelineLock,
		pipelineValidator,
		&conditions.ErrorToMessageConverter{})

	return otelReconciler, nil
}

func (r *LogPipelineController) createRequestsForAllPipelines(ctx context.Context) ([]reconcile.Request, error) {
	var pipelines telemetryv1alpha1.LogPipelineList

	var requests []reconcile.Request

	err := r.List(ctx, &pipelines)
	if err != nil {
		return nil, fmt.Errorf("failed to list LogPipelines: %w", err)
	}

	for i := range pipelines.Items {
		var pipeline = pipelines.Items[i]
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
	}

	return requests, nil
}
