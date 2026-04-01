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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
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

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	"github.com/kyma-project/telemetry-manager/internal/secretwatch"
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
	secretWatchClient    *secretwatch.Client
	pipelineLockName     types.NamespacedName
}

type TracePipelineControllerConfig struct {
	config.Global

	RestConfig         *rest.Config
	OTelCollectorImage string
}

func NewTracePipelineController(config TracePipelineControllerConfig, client client.Client, reconcileTriggerChan <-chan event.GenericEvent, secretWatchClient *secretwatch.Client) (*TracePipelineController, error) {
	pipelineCount := resourcelock.MaxPipelineCount

	if config.UnlimitedPipelines() {
		pipelineCount = resourcelock.UnlimitedPipelineCount
	}

	pipelineLock := resourcelock.NewLocker(
		client,
		types.NamespacedName{
			Name:      names.TracePipelineLock,
			Namespace: config.TargetNamespace(),
		},
		pipelineCount,
	)

	pipelineSync := resourcelock.NewSyncer(
		client,
		types.NamespacedName{
			Name:      names.TracePipelineSync,
			Namespace: config.TargetNamespace(),
		},
	)

	flowHealthProber, err := prober.NewOTelTraceGatewayProber(types.NamespacedName{Name: names.SelfMonitor, Namespace: config.TargetNamespace()})
	if err != nil {
		return nil, err
	}

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

	reconciler := tracepipeline.New(
		tracepipeline.WithClient(client),
		tracepipeline.WithGlobals(config.Global),

		tracepipeline.WithFlowHealthProber(flowHealthProber),
		tracepipeline.WithGatewayProber(&workloadstatus.DaemonSetProber{Client: client}),
		tracepipeline.WithOverridesHandler(overrides.New(config.Global, client)),
		tracepipeline.WithErrorToMessageConverter(&conditions.ErrorToMessageConverter{}),

		tracepipeline.WithPipelineLock(pipelineLock),
		tracepipeline.WithPipelineSyncer(pipelineSync),
		tracepipeline.WithPipelineValidator(pipelineValidator),
		tracepipeline.WithSecretWatcher(secretWatchClient),
	)

	return &TracePipelineController{
		Client:               client,
		reconcileTriggerChan: reconcileTriggerChan,
		reconciler:           reconciler,
		secretWatchClient:    secretWatchClient,
		pipelineLockName: types.NamespacedName{
			Name:      names.TracePipelineLock,
			Namespace: config.TargetNamespace(),
		},
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

	// Watch OTLP Gateway DaemonSet to update GatewayHealthy condition when gateway status changes
	b.Watches(
		&appsv1.DaemonSet{}, // OTLP Gateway DaemonSet
		handler.EnqueueRequestsFromMapFunc(r.mapOTLPGatewayChanges),
		ctrlbuilder.WithPredicates(ctrlpredicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetName() == names.OTLPGateway &&
				object.GetNamespace() == r.pipelineLockName.Namespace
		})),
	)

	// Watch the pipeline lock ConfigMap to trigger reconciliation of all pipelines when lock changes
	// This ensures that when a pipeline is deleted and frees up a slot, waiting pipelines get reconciled
	b.Watches(
		&corev1.ConfigMap{}, // Pipeline lock ConfigMap
		handler.EnqueueRequestsFromMapFunc(r.mapLockConfigMapChanges),
		ctrlbuilder.WithPredicates(ctrlpredicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetName() == r.pipelineLockName.Name && object.GetNamespace() == r.pipelineLockName.Namespace
		})),
	)

	// Watch for changes in Pods of interest (OTLP Gateway) to trigger reconciliation of owning pipelines.
	b.Watches(
		&corev1.Pod{},
		handler.EnqueueRequestsFromMapFunc(r.mapPodChanges),
		ctrlbuilder.WithPredicates(predicateutils.UpdateOrDelete()),
	)

	return b.Complete(r)
}

// mapLockConfigMapChanges enqueues reconciliation requests for all TracePipelines when the pipeline lock
// ConfigMap changes. This ensures that pipelines previously rejected due to the max pipeline limit get
// reconciled when slots become available.
func (r *TracePipelineController) mapLockConfigMapChanges(ctx context.Context, object client.Object) []reconcile.Request {
	logf.FromContext(ctx).V(1).Info("Pipeline lock ConfigMap changed, triggering reconciliation of all TracePipelines")
	return r.enqueueAllPipelines(ctx)
}

// mapOTLPGatewayChanges enqueues reconciliation requests for all TracePipelines when the OTLP Gateway
// DaemonSet changes. This ensures that the GatewayHealthy status condition is updated to reflect the
// current gateway state.
func (r *TracePipelineController) mapOTLPGatewayChanges(ctx context.Context, object client.Object) []reconcile.Request {
	logf.FromContext(ctx).V(1).Info("OTLP Gateway DaemonSet changed, triggering reconciliation of all TracePipelines")
	return r.enqueueAllPipelines(ctx)
}

// mapPodChanges enqueues reconciliation requests for all TracePipelines when a relevant Pod
// (OTLP Gateway) is updated or deleted. This ensures that the TelemetryFlowHealthy status condition
// is updated based on the current pod state.
func (r *TracePipelineController) mapPodChanges(ctx context.Context, object client.Object) []reconcile.Request {
	pod, ok := object.(*corev1.Pod)
	if !ok {
		logf.FromContext(ctx).V(1).Error(nil, "Unexpected type: expected Pod")
		return nil
	}

	if !isPodFrom(pod, names.OTLPGateway) {
		return nil
	}

	return r.enqueueAllPipelines(ctx)
}

// enqueueAllPipelines lists all TracePipelines and returns a reconcile request for each one.
func (r *TracePipelineController) enqueueAllPipelines(ctx context.Context) []reconcile.Request {
	var pipelineList telemetryv1beta1.TracePipelineList
	if err := r.List(ctx, &pipelineList); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to list TracePipelines")
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
