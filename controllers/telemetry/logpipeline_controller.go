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
	ctrlpredicate "sigs.k8s.io/controller-runtime/pkg/predicate"
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
	pipelineLockName     types.NamespacedName
	nodeSizeTracker      *nodesize.Tracker
}

type LogPipelineControllerConfig struct {
	config.Global

	ExporterImage              string
	FluentBitImage             string
	OTelCollectorImage         string
	ChownInitContainerImage    string
	FluentBitPriorityClassName string
	LogAgentPriorityClassName  string
	RestConfig                 *rest.Config
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

	agentFlowHealthProber, err := prober.NewOTelLogAgentProber(types.NamespacedName{Name: names.SelfMonitor, Namespace: config.TargetNamespace()})
	if err != nil {
		return nil, err
	}

	gatewayFlowHealthProber, err := prober.NewOTelLogGatewayProber(types.NamespacedName{Name: names.SelfMonitor, Namespace: config.TargetNamespace()})
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
		logpipeline.WithGlobals(config.Global),
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
		pipelineLockName: types.NamespacedName{
			Name:      names.LogPipelineLock,
			Namespace: config.TargetNamespace(),
		},
		nodeSizeTracker: nodeSizeTracker,
	}, nil
}

func (r *LogPipelineController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.reconciler.Reconcile(ctx, req)
}

// TODO: Mainly for FluentBit and Log Agent reconciliation, should be moved to agent controllers after migration.
// logPipelineOwnedResourceTypes returns the list of Kubernetes resource types that are
// managed (created/updated/deleted) by the LogPipeline reconciler and must be watched for changes.
func logPipelineOwnedResourceTypes(isIstioActive, vpaCRDExists bool) []client.Object {
	resources := []client.Object{
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

	// Only watch PeerAuthentication CR if Istio is active
	// otherwise, manager will have errors if the PeerAuthentication CRD is not present in the cluster
	if isIstioActive {
		resources = append(resources,
			&istiosecurityclientv1.PeerAuthentication{},
			&istionetworkingclientv1.DestinationRule{},
		)
	}

	// Only watch VPA CR if VPA CRD exists in the cluster
	// otherwise, manager will have errors if the VPA CRD is not present in the cluster
	// NOTE: controller needs to watch VPA CR even if the annotation to enable VPA is not present in Telemetry CR,
	// because the annotation can be added later and this function is only called once during the setup of the controller.
	if vpaCRDExists {
		resources = append(resources, &autoscalingvpav1.VerticalPodAutoscaler{})
	}

	return resources
}

func (r *LogPipelineController) SetupWithManager(mgr ctrl.Manager) error {
	b := ctrl.NewControllerManagedBy(mgr).For(&telemetryv1beta1.LogPipeline{})

	b.WatchesRawSource(
		source.Channel(r.reconcileTriggerChan, &handler.EnqueueRequestForObject{}),
	)

	ctx := context.Background()

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(mgr.GetConfig())
	if err != nil {
		return fmt.Errorf("failed to create discovery client: %w", err)
	}

	// TODO: PeerAuthentication watch should be moved to agent controllers after migration.
	isIstioActive, err := istiostatus.NewChecker(discoveryClient).IsIstioActive(ctx)
	if err != nil {
		return fmt.Errorf("failed to check Istio status: %w", err)
	}

	vpaCRDExists, err := vpastatus.NewChecker(mgr.GetConfig()).VpaCRDExists(ctx, r.Client)
	if err != nil {
		return fmt.Errorf("failed to check VPA status: %w", err)
	}

	for _, resource := range logPipelineOwnedResourceTypes(isIstioActive, vpaCRDExists) {
		b = b.Watches(
			resource,
			handler.EnqueueRequestForOwner(
				mgr.GetClient().Scheme(),
				mgr.GetRESTMapper(),
				&telemetryv1beta1.LogPipeline{},
			),
			ctrlbuilder.WithPredicates(predicateutils.OwnedResourceChanged()),
		)
	}

	// TODO: Watching the Telemetry CR should be entirely moved to the OTLP Gateway and Agents Controllers (remove this after the refactoring to LogAgent and FluentBit Controllers is done)
	// Watch Telemetry CR
	// React to spec changes (tracked by generation) and annotation changes on the Telemetry CR.
	// Annotations carry configuration like VPA opt-in that affect pipeline resources.
	// Status-only updates are ignored to avoid unnecessary reconciliation loops.
	b.Watches(
		&operatorv1beta1.Telemetry{},
		handler.EnqueueRequestsFromMapFunc(r.mapTelemetryChanges),
		ctrlbuilder.WithPredicates(ctrlpredicate.Or(ctrlpredicate.GenerationChangedPredicate{}, ctrlpredicate.AnnotationChangedPredicate{})),
	)

	// Watch for changes in Pods of interest (OTLP Gateway, Fluent Bit, Log Agent) to trigger reconciliation of owning pipelines.
	b.Watches(
		&corev1.Pod{},
		handler.EnqueueRequestsFromMapFunc(r.mapPodChanges),
		ctrlbuilder.WithPredicates(predicateutils.UpdateOrDelete()),
	)

	// Watch for changes in Nodes to track smallest node memory and trigger reconciliation of all pipelines if it changes
	b.Watches(
		&corev1.Node{},
		handler.EnqueueRequestsFromMapFunc(r.mapNodeChanges),
	)

	// Watch OTLP Gateway DaemonSet to update GatewayHealthy condition for OTLP input pipelines
	b.Watches(
		&appsv1.DaemonSet{}, // OTLP Gateway DaemonSet
		handler.EnqueueRequestsFromMapFunc(r.mapOTLPGatewayToOTLPPipelines),
		ctrlbuilder.WithPredicates(ctrlpredicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetName() == names.OTLPGateway &&
				object.GetNamespace() == r.pipelineLockName.Namespace
		})),
	)

	// Watch the pipeline lock ConfigMap to trigger reconciliation of all pipelines when lock changes
	// This ensures that when a pipeline is deleted and frees up a slot, waiting pipelines get reconciled
	b.Watches(
		&corev1.ConfigMap{}, // Pipeline lock ConfigMap
		handler.EnqueueRequestsFromMapFunc(r.mapLockConfigMapToAllPipelines),
		ctrlbuilder.WithPredicates(ctrlpredicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetName() == r.pipelineLockName.Name && object.GetNamespace() == r.pipelineLockName.Namespace
		})),
	)

	return b.Complete(r)
}

// mapLockConfigMapToAllPipelines enqueues reconciliation requests for all LogPipelines
// when the lock ConfigMap changes. This ensures that pipelines that were previously rejected
// due to max pipeline limit get a chance to acquire the lock when slots become available.
func (r *LogPipelineController) mapLockConfigMapToAllPipelines(ctx context.Context, object client.Object) []reconcile.Request {
	logf.FromContext(ctx).V(1).Info("Pipeline lock ConfigMap changed, triggering reconciliation of all LogPipelines")
	return r.enqueueAllPipelines(ctx)
}

// mapOTLPGatewayToOTLPPipelines enqueues reconciliation requests for LogPipelines with OTLP input
// when the OTLP Gateway DaemonSet changes. This ensures that GatewayHealthy status conditions
// are updated to reflect the current gateway state for pipelines that use the gateway.
func (r *LogPipelineController) mapOTLPGatewayToOTLPPipelines(ctx context.Context, object client.Object) []reconcile.Request {
	logf.FromContext(ctx).V(1).Info("OTLP Gateway DaemonSet changed, triggering reconciliation of LogPipelines with OTLP input")

	var pipelineList telemetryv1beta1.LogPipelineList
	if err := r.List(ctx, &pipelineList); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to list LogPipelines")
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, 0)

	for i := range pipelineList.Items {
		pipeline := &pipelineList.Items[i]
		// Only reconcile pipelines with OTLP input (gateway-based)
		if pipeline.Spec.Input.OTLP != nil {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: pipeline.Name,
				},
			})
		}
	}

	return requests
}

func (r *LogPipelineController) mapTelemetryChanges(ctx context.Context, object client.Object) []reconcile.Request {
	_, ok := object.(*operatorv1beta1.Telemetry)
	if !ok {
		logf.FromContext(ctx).V(1).Error(nil, "Unexpected type: expected Telemetry")
		return nil
	}

	return r.enqueueAllPipelines(ctx)
}

func (r *LogPipelineController) mapPodChanges(ctx context.Context, object client.Object) []reconcile.Request {
	pod, ok := object.(*corev1.Pod)
	if !ok {
		logf.FromContext(ctx).V(1).Error(nil, "Unexpected type: expected Pod")
		return nil
	}

	if !isPodFrom(pod, names.FluentBit, names.LogAgent, names.OTLPGateway) {
		return nil
	}

	return r.enqueueAllPipelines(ctx)
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

	agentApplierDeleter := otelcollector.NewLogAgentApplierDeleter(
		config.Global,
		config.OTelCollectorImage,
		config.LogAgentPriorityClassName)

	otelReconciler := logpipelineotel.New(
		logpipelineotel.WithClient(client),
		logpipelineotel.WithGlobals(config.Global),

		logpipelineotel.WithAgentApplierDeleter(agentApplierDeleter),
		logpipelineotel.WithAgentConfigBuilder(agentConfigBuilder),
		logpipelineotel.WithAgentFlowHealthProber(agentFlowHealthProber),
		logpipelineotel.WithGatewayFlowHealthProber(gatewayFlowHealthProber),
		logpipelineotel.WithGatewayProber(&workloadstatus.DaemonSetProber{Client: client}),
		logpipelineotel.WithAgentProber(&workloadstatus.DaemonSetProber{Client: client}),

		logpipelineotel.WithErrorToMessageConverter(&conditions.ErrorToMessageConverter{}),

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

	return r.enqueueAllPipelines(ctx)
}

// enqueueAllPipelines lists all LogPipelines and returns a reconcile request for each one.
func (r *LogPipelineController) enqueueAllPipelines(ctx context.Context) []reconcile.Request {
	var pipelineList telemetryv1beta1.LogPipelineList
	if err := r.List(ctx, &pipelineList); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to list LogPipelines")
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, len(pipelineList.Items))
	for i := range pipelineList.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name: pipelineList.Items[i].Name,
			},
		}
	}

	return requests
}
