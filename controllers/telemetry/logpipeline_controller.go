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

	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
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
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
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

	ExporterImage                string
	FluentBitImage               string
	OTelCollectorImage           string
	ChownInitContainerImage      string
	FluentBitPriorityClassName   string
	LogGatewayPriorityClassName  string
	LogAgentPriorityClassName    string
	OTLPGatewayPriorityClassName string
	RestConfig                   *rest.Config
}

func NewLogPipelineController(config LogPipelineControllerConfig, client client.Client, reconcileTriggerChan <-chan event.GenericEvent) (*LogPipelineController, error) {
	pipelineLock := resourcelock.NewLocker(
		client,
		types.NamespacedName{
			Name:      names.LogPipelineLock,
			Namespace: config.TargetNamespace(),
		},
		MaxPipelineCount,
	)

	pipelineSyncer := resourcelock.NewSyncer(
		client,
		types.NamespacedName{
			Name:      names.LogPipelineSync,
			Namespace: config.TargetNamespace(),
		},
	)

	fluentBitFlowHealthProber, err := prober.NewFluentBitProber(types.NamespacedName{Name: names.SelfMonitor, Namespace: config.TargetNamespace()})
	if err != nil {
		return nil, err
	}

	gatewayFlowHealthProber, err := prober.NewOTelLogGatewayProber(types.NamespacedName{Name: names.SelfMonitor, Namespace: config.TargetNamespace()})
	if err != nil {
		return nil, err
	}

	agentFlowHealthProber, err := prober.NewOTelLogAgentProber(types.NamespacedName{Name: names.SelfMonitor, Namespace: config.TargetNamespace()})
	if err != nil {
		return nil, err
	}

	fluentBitReconciler, err := configureFluentBitReconciler(config, client, fluentBitFlowHealthProber, pipelineLock)
	if err != nil {
		return nil, err
	}

	otelReconciler, err := configureOTelReconciler(config, client, pipelineLock, gatewayFlowHealthProber, agentFlowHealthProber)
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
	b := ctrl.NewControllerManagedBy(mgr).For(&telemetryv1beta1.LogPipeline{})

	b.WatchesRawSource(
		source.Channel(r.reconcileTriggerChan, &handler.EnqueueRequestForObject{}),
	)

	ownedResourceTypesToWatch := []client.Object{
		&appsv1.DaemonSet{},
		&appsv1.Deployment{},
		&corev1.ConfigMap{},
		&corev1.Pod{},
		&corev1.Secret{},
		&corev1.Service{},
		&corev1.ServiceAccount{},
		&rbacv1.ClusterRole{},
		&rbacv1.ClusterRoleBinding{},
		&networkingv1.NetworkPolicy{},
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(mgr.GetConfig())
	if err != nil {
		return fmt.Errorf("failed to create discovery client: %w", err)
	}

	isIstioActive := istiostatus.NewChecker(discoveryClient).IsIstioActive(context.Background())

	if isIstioActive {
		ownedResourceTypesToWatch = append(ownedResourceTypesToWatch, &istiosecurityclientv1.PeerAuthentication{})
	}

	for _, resource := range ownedResourceTypesToWatch {
		b = b.Watches(
			resource,
			handler.EnqueueRequestForOwner(mgr.GetClient().Scheme(),
				mgr.GetRESTMapper(),
				&telemetryv1beta1.LogPipeline{},
			),
			ctrlbuilder.WithPredicates(predicateutils.OwnedResourceChanged()),
		)
	}

	return b.Watches(
		&operatorv1beta1.Telemetry{},
		handler.EnqueueRequestsFromMapFunc(r.mapTelemetryChanges),
		ctrlbuilder.WithPredicates(predicateutils.CreateOrUpdateOrDelete()),
	).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.mapSecretChanges),
			ctrlbuilder.WithPredicates(predicateutils.CreateOrUpdateOrDelete()),
		).Complete(r)
}

func (r *LogPipelineController) mapTelemetryChanges(ctx context.Context, object client.Object) []reconcile.Request {
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

	prober := func() logpipelineotel.Prober {
		if config.DeployOTLPGateway() {
			return &workloadstatus.DaemonSetProber{Client: client}
		}

		return &workloadstatus.DeploymentProber{Client: client}
	}()

	agentApplierDeleter := otelcollector.NewLogAgentApplierDeleter(
		config.Global,
		config.OTelCollectorImage,
		config.LogAgentPriorityClassName)

	gatewayAppliedDeleter := otelcollector.NewLogGatewayApplierDeleter(
		config.Global,
		config.OTelCollectorImage,
		config.LogGatewayPriorityClassName)

	// Create OTLP gateway applier deleter for DaemonSet mode
	otlpGatewayApplierDeleter := otelcollector.NewOTLPGatewayApplierDeleter(
		config.Global,
		config.OTelCollectorImage,
		config.OTLPGatewayPriorityClassName)

	otelReconciler := logpipelineotel.New(
		logpipelineotel.WithClient(client),
		logpipelineotel.WithGlobals(config.Global),

		logpipelineotel.WithAgentApplierDeleter(agentApplierDeleter),
		logpipelineotel.WithAgentConfigBuilder(agentConfigBuilder),
		logpipelineotel.WithAgentFlowHealthProber(agentFlowHealthProber),
		logpipelineotel.WithAgentProber(&workloadstatus.DaemonSetProber{Client: client}),

		logpipelineotel.WithErrorToMessageConverter(&conditions.ErrorToMessageConverter{}),

		logpipelineotel.WithGatewayApplierDeleter(gatewayAppliedDeleter),
		logpipelineotel.WithOTLPGatewayApplierDeleter(otlpGatewayApplierDeleter),
		logpipelineotel.WithGatewayConfigBuilder(&loggateway.Builder{Reader: client}),
		logpipelineotel.WithGatewayFlowHealthProber(gatewayFlowHealthProber),
		logpipelineotel.WithGatewayProber(prober),

		logpipelineotel.WithIstioStatusChecker(istiostatus.NewChecker(discoveryClient)),
		logpipelineotel.WithPipelineLock(pipelineLock),
		logpipelineotel.WithPipelineValidator(pipelineValidator),
	)

	return otelReconciler, nil
}

func (r *LogPipelineController) createRequestsForAllPipelines(ctx context.Context) ([]reconcile.Request, error) {
	var pipelines telemetryv1beta1.LogPipelineList

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

func (r *LogPipelineController) mapSecretChanges(ctx context.Context, object client.Object) []reconcile.Request {
	var pipelines telemetryv1beta1.LogPipelineList

	var requests []reconcile.Request

	err := r.List(ctx, &pipelines)
	if err != nil {
		logf.FromContext(ctx).Error(err, "failed to list LogPipelines")
		return requests
	}

	secret, ok := object.(*corev1.Secret)
	if !ok {
		logf.FromContext(ctx).Error(nil, fmt.Sprintf("expected Secret object but got: %T", object))
		return requests
	}

	for i := range pipelines.Items {
		var pipeline = pipelines.Items[i]

		if r.referencesSecret(secret.Name, secret.Namespace, &pipeline) {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: pipeline.Name}})
		}
	}

	return requests
}

func (r *LogPipelineController) referencesSecret(secretName, secretNamespace string, pipeline *telemetryv1beta1.LogPipeline) bool {
	refs := secretref.GetLogPipelineRefs(pipeline)
	for _, ref := range refs {
		if ref.Name == secretName && ref.Namespace == secretNamespace {
			return true
		}
	}

	return false
}
