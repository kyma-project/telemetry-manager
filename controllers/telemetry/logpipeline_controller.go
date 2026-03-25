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

	istionetworkingclientv1 "istio.io/client-go/pkg/apis/networking/v1"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	autoscalingvpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	"github.com/kyma-project/telemetry-manager/internal/istiostatus"
	"github.com/kyma-project/telemetry-manager/internal/nodesize"
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
	"github.com/kyma-project/telemetry-manager/internal/secretwatch"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	predicateutils "github.com/kyma-project/telemetry-manager/internal/utils/predicate"
	"github.com/kyma-project/telemetry-manager/internal/validators/endpoint"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
	"github.com/kyma-project/telemetry-manager/internal/vpastatus"
	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

// LogPipelineController reconciles a LogPipeline object
type LogPipelineController struct {
	client.Client

	globals config.Global

	reconcileTriggerChan <-chan event.GenericEvent
	reconciler           *logpipeline.Reconciler
	secretWatchClient    *secretwatch.Client
	nodeSizeTracker      *nodesize.Tracker
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

func NewLogPipelineController(config LogPipelineControllerConfig, client client.Client, reconcileTriggerChan <-chan event.GenericEvent, secretWatchClient *secretwatch.Client, nodeSizeTracker *nodesize.Tracker) (*LogPipelineController, error) {
	pipelineCount := resourcelock.MaxPipelineCount

	if config.UnlimitedPipelines() {
		pipelineCount = resourcelock.UnlimitedPipelineCount
	}

	pipelineLockOTEL := resourcelock.NewLocker(
		client,
		types.NamespacedName{
			Name:      names.LogPipelineLock,
			Namespace: config.TargetNamespace(),
		},
		pipelineCount,
	)

	pipelineLock := resourcelock.NewLocker(
		client,
		types.NamespacedName{
			Name:      names.LogPipelineLock,
			Namespace: config.TargetNamespace(),
		},
		resourcelock.MaxPipelineCount,
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

	otelReconciler, err := configureOTelReconciler(config, client, pipelineLockOTEL, gatewayFlowHealthProber, agentFlowHealthProber, nodeSizeTracker)
	if err != nil {
		return nil, err
	}

	reconciler := logpipeline.New(
		client,
		logpipeline.WithOverridesHandler(overrides.New(config.Global, client)),
		logpipeline.WithPipelineSyncer(pipelineSyncer),
		logpipeline.WithReconcilers(fluentBitReconciler, otelReconciler),
		logpipeline.WithSecretWatcher(secretWatchClient),
	)

	return &LogPipelineController{
		Client:               client,
		globals:              config.Global,
		reconcileTriggerChan: reconcileTriggerChan,
		reconciler:           reconciler,
		secretWatchClient:    secretWatchClient,
		nodeSizeTracker:      nodeSizeTracker,
	}, nil
}

func (r *LogPipelineController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.reconciler.Reconcile(ctx, req)
}

// logPipelineOwnedResourceTypes returns the list of Kubernetes resource types that are always
// managed (created/updated/deleted) by the LogPipeline reconciler and must be watched for changes.
// Conditionally managed resources (PeerAuthentication, DestinationRule, VPA) are handled separately
// in SetupWithManager based on runtime cluster capabilities.
func logPipelineOwnedResourceTypes() []client.Object {
	return []client.Object{
		&appsv1.DaemonSet{},
		&appsv1.Deployment{},
		&corev1.ConfigMap{},
		&corev1.Secret{},
		&corev1.Service{},
		&corev1.ServiceAccount{},
		&rbacv1.ClusterRole{},
		&rbacv1.ClusterRoleBinding{},
		&networkingv1.NetworkPolicy{},
	}
}

func (r *LogPipelineController) SetupWithManager(mgr ctrl.Manager) error {
	b := ctrl.NewControllerManagedBy(mgr).For(&telemetryv1beta1.LogPipeline{})

	b.WatchesRawSource(
		source.Channel(r.reconcileTriggerChan, &handler.EnqueueRequestForObject{}),
	)

	ownedResourceTypesToWatch := logPipelineOwnedResourceTypes()

	ctx := context.Background()

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(mgr.GetConfig())
	if err != nil {
		return fmt.Errorf("failed to create discovery client: %w", err)
	}

	isIstioActive, err := istiostatus.NewChecker(discoveryClient).IsIstioActive(ctx)
	if err != nil {
		return fmt.Errorf("failed to check Istio status: %w", err)
	}

	vpaCRDExists, err := vpastatus.NewChecker(mgr.GetConfig()).VpaCRDExists(ctx, r.Client)
	if err != nil {
		return fmt.Errorf("failed to check VPA status: %w", err)
	}

	// Only watch Istio CRs if Istio is active
	// otherwise, manager will have errors if the CRDs are not present in the cluster
	if isIstioActive {
		ownedResourceTypesToWatch = append(ownedResourceTypesToWatch,
			&istiosecurityclientv1.PeerAuthentication{},
			&istionetworkingclientv1.DestinationRule{},
		)
	}

	// Only watch VPA CR if VPA CRD exists in the cluster
	// otherwise, manager will have errors if the VPA CRD is not present in the cluster
	// NOTE: controller needs to watch VPA CR even if the annotation to enable VPA is not present in Telemetry CR,
	// because the annotation can be added later and this function is only called once during the setup of the controller.
	if vpaCRDExists {
		ownedResourceTypesToWatch = append(ownedResourceTypesToWatch, &autoscalingvpav1.VerticalPodAutoscaler{})
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
		ctrlbuilder.WithPredicates(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})),
	).Watches(
		&corev1.Pod{},
		handler.EnqueueRequestsFromMapFunc(r.mapPodChanges),
		ctrlbuilder.WithPredicates(predicateutils.UpdateOrDelete()),
	).Watches(
		&corev1.Node{},
		handler.EnqueueRequestsFromMapFunc(r.mapNodeChanges),
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

func (r *LogPipelineController) mapPodChanges(ctx context.Context, object client.Object) []reconcile.Request {
	pod, ok := object.(*corev1.Pod)
	if !ok {
		logf.FromContext(ctx).V(1).Error(nil, "Unexpected type: expected Pod")
		return nil
	}

	if !isPodFrom(pod, names.FluentBit, names.LogGateway, names.LogAgent, names.OTLPGateway) {
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
func configureOTelReconciler(config LogPipelineControllerConfig, client client.Client, pipelineLock logpipelineotel.PipelineLock, gatewayFlowHealthProber *prober.OTelGatewayProber, agentFlowHealthProber *prober.OTelAgentProber, nodeSizeTracker *nodesize.Tracker) (*logpipelineotel.Reconciler, error) {
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
		logpipelineotel.WithVpaStatusChecker(vpastatus.NewChecker(config.RestConfig)),
		logpipelineotel.WithNodeSizeTracker(nodeSizeTracker),
		logpipelineotel.WithPipelineLock(pipelineLock),
		logpipelineotel.WithPipelineValidator(pipelineValidator),
	)

	return otelReconciler, nil
}

func (r *LogPipelineController) mapNodeChanges(ctx context.Context, object client.Object) []reconcile.Request {
	changed, err := r.nodeSizeTracker.UpdateSmallestMemory(ctx)
	if err != nil {
		logf.FromContext(ctx).Error(err, "Unable to update smallest node memory")
		return nil
	}

	if !changed {
		return nil
	}

	requests, err := r.createRequestsForAllPipelines(ctx)
	if err != nil {
		logf.FromContext(ctx).Error(err, "Unable to create reconcile requests")
	}

	return requests
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
