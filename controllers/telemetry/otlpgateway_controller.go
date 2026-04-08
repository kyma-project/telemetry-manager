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
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/istiostatus"
	"github.com/kyma-project/telemetry-manager/internal/nodesize"
	otlpgatewayconfig "github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpgateway" //nolint:importas // needed to disambiguate from reconciler package
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	otlpgatewayreconciler "github.com/kyma-project/telemetry-manager/internal/reconciler/otlpgateway" //nolint:importas // needed to disambiguate from config package
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	predicateutils "github.com/kyma-project/telemetry-manager/internal/utils/predicate"
	"github.com/kyma-project/telemetry-manager/internal/vpastatus"
)

// OTLPGatewayController reconciles the OTLP Gateway DaemonSet based on pipeline references
type OTLPGatewayController struct {
	client.Client

	reconcileTriggerChan <-chan event.GenericEvent
	reconciler           *otlpgatewayreconciler.Reconciler
	nodeSizeTracker      *nodesize.Tracker
}

type OTLPGatewayControllerConfig struct {
	config.Global

	RestConfig                   *rest.Config
	OTelCollectorImage           string
	OTLPGatewayPriorityClassName string
}

func NewOTLPGatewayController(config OTLPGatewayControllerConfig, client client.Client, reconcileTriggerChan <-chan event.GenericEvent, nodeSizeTracker *nodesize.Tracker) (*OTLPGatewayController, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config.RestConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}

	reconciler := otlpgatewayreconciler.NewReconciler(
		client,
		otlpgatewayreconciler.WithGlobals(config.Global),
		otlpgatewayreconciler.WithOverridesHandler(overrides.New(config.Global, client)),
		otlpgatewayreconciler.WithGatewayApplierDeleter(
			otelcollector.NewOTLPGatewayApplierDeleter(
				config.Global,
				config.OTelCollectorImage,
				config.OTLPGatewayPriorityClassName,
			),
		),
		otlpgatewayreconciler.WithConfigBuilder(&otlpgatewayconfig.Builder{Reader: client}),
		otlpgatewayreconciler.WithIstioStatusChecker(istiostatus.NewChecker(discoveryClient)),
	)

	return &OTLPGatewayController{
		Client:               client,
		reconcileTriggerChan: reconcileTriggerChan,
		reconciler:           reconciler,
		nodeSizeTracker:      nodeSizeTracker,
	}, nil
}

func (r *OTLPGatewayController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.reconciler.Reconcile(ctx, req)
}

// otlpGatewayOwnedResourceTypes returns the list of Kubernetes resource types that are
// managed (created/updated/deleted) by the OTLP Gateway reconciler and must be watched for changes.
func otlpGatewayOwnedResourceTypes(isIstioActive, vpaCRDExists bool) []client.Object {
	resources := []client.Object{
		&appsv1.DaemonSet{},
		&corev1.ConfigMap{},
		&corev1.Secret{},
		&corev1.Service{},
		&corev1.ServiceAccount{},
		&rbacv1.ClusterRole{},
		&rbacv1.ClusterRoleBinding{},
		&rbacv1.Role{},
		&rbacv1.RoleBinding{},
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

func (r *OTLPGatewayController) SetupWithManager(mgr ctrl.Manager) error {
	// Primary watch: filter to only the coordination ConfigMap written by pipeline controllers
	b := ctrl.NewControllerManagedBy(mgr).For(&corev1.ConfigMap{},
		ctrlbuilder.WithPredicates(
			ctrlpredicate.NewPredicateFuncs(func(obj client.Object) bool {
				return obj.GetName() == names.OTLPGatewayCoordinationConfigMap
			}),
		),
	)

	// Watch reconcile trigger channel
	b.WatchesRawSource(
		source.Channel(r.reconcileTriggerChan, &handler.EnqueueRequestForObject{}),
	)

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

	for _, resource := range otlpGatewayOwnedResourceTypes(isIstioActive, vpaCRDExists) {
		b = b.Watches(
			resource,
			handler.EnqueueRequestsFromMapFunc(r.mapOwnedResourceChanges),
			ctrlbuilder.WithPredicates(predicateutils.OwnedResourceChanged()),
		)
	}

	// Watch Telemetry CR
	// React to spec changes (tracked by generation) and annotation changes on the Telemetry CR.
	// Annotations carry configuration like VPA opt-in that affect gateway resources.
	// Status-only updates are ignored to avoid unnecessary reconciliation loops.
	b.Watches(
		&operatorv1beta1.Telemetry{},
		handler.EnqueueRequestsFromMapFunc(r.mapTelemetryChanges),
		ctrlbuilder.WithPredicates(ctrlpredicate.Or(ctrlpredicate.GenerationChangedPredicate{}, ctrlpredicate.AnnotationChangedPredicate{})),
	)

	// Watch for changes in Nodes to track smallest node memory and trigger reconciliation if it changes
	b.Watches(
		&corev1.Node{},
		handler.EnqueueRequestsFromMapFunc(r.mapNodeChanges),
	)

	return b.Complete(r)
}

// mapTelemetryChanges enqueues a reconciliation request for the OTLP Gateway coordination ConfigMap
// when the Telemetry CR changes. This ensures the gateway configuration reflects updated module-level
// settings such as VPA opt-in annotations.
func (r *OTLPGatewayController) mapTelemetryChanges(ctx context.Context, object client.Object) []reconcile.Request {
	_, ok := object.(*operatorv1beta1.Telemetry)
	if !ok {
		logf.FromContext(ctx).V(1).Error(nil, "Unexpected type: expected Telemetry")
		return nil
	}

	return r.enqueueConfigMap()
}

// mapOwnedResourceChanges enqueues a reconciliation request for the OTLP Gateway coordination ConfigMap
// when an owned resource (DaemonSet, ConfigMap, Secret, Service, etc.) is externally modified.
// This ensures the reconciler can restore the desired state of gateway resources.
func (r *OTLPGatewayController) mapOwnedResourceChanges(ctx context.Context, object client.Object) []reconcile.Request {
	logf.FromContext(ctx).V(1).Info("owned resource changed, triggering OTLP gateway reconciliation", "resource", object.GetName())
	return r.enqueueConfigMap()
}

// mapNodeChanges updates the node size tracker when a Node is added, removed, or modified.
// If the smallest node memory changes, it enqueues a reconciliation request for the OTLP Gateway
// coordination ConfigMap so that resource requirements can be recalculated.
func (r *OTLPGatewayController) mapNodeChanges(ctx context.Context, object client.Object) []reconcile.Request {
	changed, err := r.nodeSizeTracker.UpdateSmallestMemory(ctx)
	if err != nil {
		logf.FromContext(ctx).Error(err, "Unable to update smallest node memory")
		return nil
	}

	if !changed {
		return nil
	}

	return r.enqueueConfigMap()
}

// enqueueConfigMap returns a reconcile request for the OTLP Gateway coordination ConfigMap.
func (r *OTLPGatewayController) enqueueConfigMap() []reconcile.Request {
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      names.OTLPGatewayCoordinationConfigMap,
				Namespace: r.reconciler.Globals().TargetNamespace(),
			},
		},
	}
}
