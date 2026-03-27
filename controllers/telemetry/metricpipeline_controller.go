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
	"github.com/kyma-project/telemetry-manager/internal/istiostatus"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metricagent"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/metricpipeline"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
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

// MetricPipelineController reconciles a MetricPipeline object
type MetricPipelineController struct {
	client.Client

	globals config.Global

	reconcileTriggerChan <-chan event.GenericEvent
	reconciler           *metricpipeline.Reconciler
	secretWatchClient    *secretwatch.Client
	pipelineLockName     types.NamespacedName
}

type MetricPipelineControllerConfig struct {
	config.Global

	MetricAgentPriorityClassName string
	OTelCollectorImage           string
	RestConfig                   *rest.Config
}

func NewMetricPipelineController(config MetricPipelineControllerConfig, client client.Client, reconcileTriggerChan <-chan event.GenericEvent, secretWatchClient *secretwatch.Client) (*MetricPipelineController, error) {
	pipelineCount := resourcelock.MaxPipelineCount

	if config.UnlimitedPipelines() {
		pipelineCount = resourcelock.UnlimitedPipelineCount
	}

	pipelineLock := resourcelock.NewLocker(
		client,
		types.NamespacedName{
			Name:      names.MetricPipelineLock,
			Namespace: config.TargetNamespace(),
		},
		pipelineCount,
	)

	pipelineSync := resourcelock.NewSyncer(
		client,
		types.NamespacedName{
			Name:      names.MetricPipelineSync,
			Namespace: config.TargetNamespace(),
		},
	)

	gatewayFlowHealthProber, err := prober.NewOTelMetricGatewayProber(types.NamespacedName{Name: names.SelfMonitor, Namespace: config.TargetNamespace()})
	if err != nil {
		return nil, err
	}

	agentFlowHealthProber, err := prober.NewOTelMetricAgentProber(types.NamespacedName{Name: names.SelfMonitor, Namespace: config.TargetNamespace()})
	if err != nil {
		return nil, err
	}

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

	reconciler := metricpipeline.New(
		metricpipeline.WithClient(client),
		metricpipeline.WithGlobals(config.Global),

		metricpipeline.WithAgentApplierDeleter(otelcollector.NewMetricAgentApplierDeleter(config.Global, config.OTelCollectorImage, config.MetricAgentPriorityClassName)),
		metricpipeline.WithAgentConfigBuilder(agentConfigBuilder),
		metricpipeline.WithAgentFlowHealthProber(agentFlowHealthProber),
		metricpipeline.WithAgentProber(&workloadstatus.DaemonSetProber{Client: client}),

		metricpipeline.WithGatewayFlowHealthProber(gatewayFlowHealthProber),
		metricpipeline.WithGatewayProber(&workloadstatus.DaemonSetProber{Client: client}),

		metricpipeline.WithErrorToMessageConverter(&conditions.ErrorToMessageConverter{}),
		metricpipeline.WithIstioStatusChecker(istiostatus.NewChecker(discoveryClient)),
		metricpipeline.WithVpaStatusChecker(vpastatus.NewChecker(config.RestConfig)),
		metricpipeline.WithOverridesHandler(overrides.New(config.Global, client)),
		metricpipeline.WithPipelineValidator(pipelineValidator),

		metricpipeline.WithPipelineLock(pipelineLock),
		metricpipeline.WithPipelineSyncer(pipelineSync),
		metricpipeline.WithSecretWatcher(secretWatchClient),
	)

	return &MetricPipelineController{
		Client:               client,
		globals:              config.Global,
		reconcileTriggerChan: reconcileTriggerChan,
		reconciler:           reconciler,
		secretWatchClient:    secretWatchClient,
		pipelineLockName: types.NamespacedName{
			Name:      names.MetricPipelineLock,
			Namespace: config.TargetNamespace(),
		},
	}, nil
}

func (r *MetricPipelineController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.reconciler.Reconcile(ctx, req)
}

// SetupWithManager sets up the controller with the Manager.
func (r *MetricPipelineController) SetupWithManager(mgr ctrl.Manager) error {
	b := ctrl.NewControllerManagedBy(mgr).For(&telemetryv1beta1.MetricPipeline{})

	b.WatchesRawSource(
		source.Channel(r.reconcileTriggerChan, &handler.EnqueueRequestForObject{}),
	)

	// TODO: Mainly for Metric Agent reconciliation, should be moved to agent controller after migration.
	ownedResourceTypesToWatch := []client.Object{
		&appsv1.DaemonSet{},
		&corev1.ConfigMap{},
		&corev1.Pod{},
		&corev1.Secret{},
		&corev1.Service{},
		&corev1.ServiceAccount{},
		&rbacv1.ClusterRole{},
		&rbacv1.ClusterRoleBinding{},
		&rbacv1.Role{},
		&rbacv1.RoleBinding{},
		&networkingv1.NetworkPolicy{},
	}

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

	// TODO: PeerAuthentication watch should be moved to agent controller after migration.
	// Only watch PeerAuthentication CR if Istio is active
	// otherwise, manager will have errors if the PeerAuthentication CRD is not present in the cluster
	if isIstioActive {
		ownedResourceTypesToWatch = append(ownedResourceTypesToWatch, &istiosecurityclientv1.PeerAuthentication{})
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
				&telemetryv1beta1.MetricPipeline{},
			),
			ctrlbuilder.WithPredicates(predicateutils.OwnedResourceChanged()),
		)
	}

	// Watch OTLP Gateway DaemonSet to update GatewayHealthy condition when gateway status changes
	b.Watches(
		&appsv1.DaemonSet{}, // OTLP Gateway DaemonSet
		handler.EnqueueRequestsFromMapFunc(r.mapOTLPGatewayToAllPipelines),
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

	// TODO: Watching the Telemetry CR should be entirely moved to the OTLP Gateway and Agents Controllers (remove this after the refactoring to MetricAgent Controller is done)
	return b.Watches(
		&operatorv1beta1.Telemetry{},
		handler.EnqueueRequestsFromMapFunc(r.mapTelemetryChanges),
		ctrlbuilder.WithPredicates(predicateutils.CreateOrUpdateOrDelete()),
	).Complete(r)
}

// mapLockConfigMapToAllPipelines enqueues reconciliation requests for all MetricPipelines
// when the lock ConfigMap changes. This ensures that pipelines that were previously rejected
// due to max pipeline limit get a chance to acquire the lock when slots become available.
func (r *MetricPipelineController) mapLockConfigMapToAllPipelines(ctx context.Context, object client.Object) []reconcile.Request {
	logf.FromContext(ctx).V(1).Info("Pipeline lock ConfigMap changed, triggering reconciliation of all MetricPipelines")
	return r.enqueueAllPipelines(ctx)
}

// mapOTLPGatewayToAllPipelines enqueues reconciliation requests for all MetricPipelines
// when the OTLP Gateway DaemonSet changes. This ensures that GatewayHealthy status conditions
// are updated to reflect the current gateway state.
func (r *MetricPipelineController) mapOTLPGatewayToAllPipelines(ctx context.Context, object client.Object) []reconcile.Request {
	logf.FromContext(ctx).V(1).Info("OTLP Gateway DaemonSet changed, triggering reconciliation of all MetricPipelines")
	return r.enqueueAllPipelines(ctx)
}

func (r *MetricPipelineController) mapTelemetryChanges(ctx context.Context, object client.Object) []reconcile.Request {
	_, ok := object.(*operatorv1beta1.Telemetry)
	if !ok {
		logf.FromContext(ctx).V(1).Error(nil, "Unexpected type: expected Telemetry")
		return nil
	}

	return r.enqueueAllPipelines(ctx)
}

// enqueueAllPipelines lists all MetricPipelines and returns a reconcile request for each one.
func (r *MetricPipelineController) enqueueAllPipelines(ctx context.Context) []reconcile.Request {
	var pipelineList telemetryv1beta1.MetricPipelineList
	if err := r.List(ctx, &pipelineList); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to list MetricPipelines")
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
