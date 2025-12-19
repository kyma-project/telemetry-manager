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
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	"github.com/kyma-project/telemetry-manager/internal/istiostatus"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/logagent"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/loggateway"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline"
	logpipelinefluentbit "github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/fluentbit"
	logpipelineotel "github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/otel"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"
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

// LogPipelineController reconciles a LogPipeline object
type LogPipelineController struct {
	client.Client

	reconcileTriggerChan <-chan event.GenericEvent
	reconciler           *logpipeline.Reconciler
}

type LogPipelineControllerConfig struct {
	config.Global

	ExporterImage               string
	FluentBitImage              string
	OTelCollectorImage          string
	ChownInitContainerImage     string
	FluentBitPriorityClassName  string
	LogGatewayPriorityClassName string
	LogAgentPriorityClassName   string
	RestConfig                  *rest.Config
}

func NewLogPipelineController(config LogPipelineControllerConfig, client client.Client, reconcileTriggerChan <-chan event.GenericEvent) (*LogPipelineController, error) {
	pipelineLock := resourcelock.NewLocker(
		client,
		types.NamespacedName{
			Name:      "telemetry-logpipeline-lock",
			Namespace: config.TargetNamespace(),
		},
		MaxPipelineCount,
	)

	pipelineSyncer := resourcelock.NewSyncer(
		client,
		types.NamespacedName{
			Name:      "telemetry-logpipeline-sync",
			Namespace: config.TargetNamespace(),
		},
	)

	fluentBitFlowHealthProber, err := prober.NewFluentBitProber(types.NamespacedName{Name: selfmonitor.ServiceName, Namespace: config.TargetNamespace()})
	if err != nil {
		return nil, err
	}

	otelGatewayFlowHealthProber, err := prober.NewOTelLogGatewayProber(types.NamespacedName{Name: selfmonitor.ServiceName, Namespace: config.TargetNamespace()})
	if err != nil {
		return nil, err
	}

	otelAgentFlowHealthProber, err := prober.NewOTelLogAgentProber(types.NamespacedName{Name: selfmonitor.ServiceName, Namespace: config.TargetNamespace()})
	if err != nil {
		return nil, err
	}

	fluentBitReconciler, err := configureFluentBitReconciler(config, client, fluentBitFlowHealthProber, pipelineLock)
	if err != nil {
		return nil, err
	}

	otelReconciler, err := configureOTelReconciler(config, client, pipelineLock, otelGatewayFlowHealthProber, otelAgentFlowHealthProber)
	if err != nil {
		return nil, err
	}

	reconciler := logpipeline.New(
		client,
		logpipeline.WithOverridesHandler(overrides.New(config.Global, client)),
		logpipeline.WithPipelineSyncer(pipelineSyncer),
		logpipeline.WithReconcilers(fluentBitReconciler, otelReconciler),
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

func configureFluentBitReconciler(config LogPipelineControllerConfig, client client.Client, flowHealthProber *prober.FluentBitProber, pipelineLock logpipelinefluentbit.PipelineLock) (*logpipelinefluentbit.Reconciler, error) {
	pipelineValidator := logpipelinefluentbit.NewValidator(
		logpipelinefluentbit.WithEndpointValidator(&endpoint.Validator{Client: client}),
		logpipelinefluentbit.WithTLSCertValidator(tlscert.New(client)),
		logpipelinefluentbit.WithSecretRefValidator(&secretref.Validator{Client: client}),
		logpipelinefluentbit.WithValidatorPipelineLock(pipelineLock),
	)

	fluentBitApplierDeleter := fluentbit.NewFluentBitApplierDeleter(
		config.Global,
		config.TargetNamespace(),
		config.FluentBitImage,
		config.ExporterImage,
		config.ChownInitContainerImage,
		config.FluentBitPriorityClassName,
	)

	fluentBitConfigBuilder := builder.NewFluentBitConfigBuilder(client)

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config.RestConfig)
	if err != nil {
		return nil, err
	}

	fbReconciler := logpipelinefluentbit.New(
		logpipelinefluentbit.WithClient(client),
		logpipelinefluentbit.WithGlobals(config.Global),

		logpipelinefluentbit.WithAgentApplierDeleter(fluentBitApplierDeleter),
		logpipelinefluentbit.WithAgentConfigBuilder(fluentBitConfigBuilder),
		logpipelinefluentbit.WithAgentProber(&workloadstatus.DaemonSetProber{Client: client}),

		logpipelinefluentbit.WithErrorToMessageConverter(&conditions.ErrorToMessageConverter{}),
		logpipelinefluentbit.WithFlowHealthProber(flowHealthProber),
		logpipelinefluentbit.WithIstioStatusChecker(istiostatus.NewChecker(discoveryClient)),
		logpipelinefluentbit.WithPipelineLock(pipelineLock),
		logpipelinefluentbit.WithPipelineValidator(pipelineValidator),
	)

	return fbReconciler, nil
}

//nolint:unparam // error is always nil: An error could be returned after implementing the IstioStatusChecker (TODO)
func configureOTelReconciler(config LogPipelineControllerConfig, client client.Client, pipelineLock logpipelineotel.PipelineLock, gatewayFlowHealthProber *prober.OTelGatewayProber, agentFlowHealthProber *prober.OTelAgentProber) (*logpipelineotel.Reconciler, error) {
	transformSpecValidator, err := ottl.NewTransformSpecValidator(ottl.SignalTypeLog)
	if err != nil {
		return nil, err
	}

	filterSpecValidator, err := ottl.NewFilterSpecValidator(ottl.SignalTypeLog)
	if err != nil {
		return nil, err
	}

	pipelineValidator := logpipelineotel.NewValidator(
		logpipelineotel.WithValidatorPipelineLock(pipelineLock),
		logpipelineotel.WithEndpointValidator(&endpoint.Validator{Client: client}),
		logpipelineotel.WithTLSCertValidator(tlscert.New(client)),
		logpipelineotel.WithSecretRefValidator(&secretref.Validator{Client: client}),
		logpipelineotel.WithTransformSpecValidator(transformSpecValidator),
		logpipelineotel.WithFilterSpecValidator(filterSpecValidator),
	)

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config.RestConfig)
	if err != nil {
		return nil, err
	}

	agentConfigBuilder := &logagent.Builder{
		Reader: client,
	}

	agentApplierDeleter := otelcollector.NewLogAgentApplierDeleter(
		config.Global,
		config.OTelCollectorImage,
		config.LogAgentPriorityClassName)

	gatewayAppliedDeleter := otelcollector.NewLogGatewayApplierDeleter(
		config.Global,
		config.OTelCollectorImage,
		config.LogGatewayPriorityClassName)

	otelReconciler := logpipelineotel.New(
		logpipelineotel.WithClient(client),
		logpipelineotel.WithGlobals(config.Global),

		logpipelineotel.WithAgentApplierDeleter(agentApplierDeleter),
		logpipelineotel.WithAgentConfigBuilder(agentConfigBuilder),
		logpipelineotel.WithAgentFlowHealthProber(agentFlowHealthProber),
		logpipelineotel.WithAgentProber(&workloadstatus.DaemonSetProber{Client: client}),

		logpipelineotel.WithErrorToMessageConverter(&conditions.ErrorToMessageConverter{}),

		logpipelineotel.WithGatewayApplierDeleter(gatewayAppliedDeleter),
		logpipelineotel.WithGatewayConfigBuilder(&loggateway.Builder{Reader: client}),
		logpipelineotel.WithGatewayFlowHealthProber(gatewayFlowHealthProber),
		logpipelineotel.WithGatewayProber(&workloadstatus.DeploymentProber{Client: client}),

		logpipelineotel.WithIstioStatusChecker(istiostatus.NewChecker(discoveryClient)),
		logpipelineotel.WithPipelineLock(pipelineLock),
		logpipelineotel.WithPipelineValidator(pipelineValidator),
	)

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
