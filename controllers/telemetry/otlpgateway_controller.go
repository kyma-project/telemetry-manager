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
	otlpgatewayconfig "github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpgateway" //nolint:importas // needed to disambiguate from reconciler package
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	otlpgatewayreconciler "github.com/kyma-project/telemetry-manager/internal/reconciler/otlpgateway" //nolint:importas // needed to disambiguate from config package
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	predicateutils "github.com/kyma-project/telemetry-manager/internal/utils/predicate"
)

// OTLPGatewayController reconciles the OTLP Gateway DaemonSet based on pipeline references
type OTLPGatewayController struct {
	client.Client

	reconcileTriggerChan <-chan event.GenericEvent
	reconciler           *otlpgatewayreconciler.Reconciler
}

type OTLPGatewayControllerConfig struct {
	config.Global

	RestConfig                   *rest.Config
	OTelCollectorImage           string
	OTLPGatewayPriorityClassName string
}

func NewOTLPGatewayController(config OTLPGatewayControllerConfig, client client.Client, reconcileTriggerChan <-chan event.GenericEvent) (*OTLPGatewayController, error) {
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
	}, nil
}

func (r *OTLPGatewayController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.reconciler.Reconcile(ctx, req)
}

func (r *OTLPGatewayController) SetupWithManager(mgr ctrl.Manager) error {
	// Primary watch: filter to only the coordination ConfigMap written by pipeline controllers
	b := ctrl.NewControllerManagedBy(mgr).For(&corev1.ConfigMap{},
		ctrlbuilder.WithPredicates(
			predicateutils.CreateOrUpdateOrDelete(),
			ctrlpredicate.NewPredicateFuncs(func(obj client.Object) bool {
				return obj.GetName() == names.OTLPGatewayPipelinesSyncConfigMap
			}),
		),
	)

	// Watch reconcile trigger channel
	b.WatchesRawSource(
		source.Channel(r.reconcileTriggerChan, &handler.EnqueueRequestForObject{}),
	)

	// Watch owned resources (DaemonSet and associated resources)
	ownedResourceTypesToWatch := []client.Object{
		&appsv1.DaemonSet{},           // OTLP Gateway DaemonSet
		&corev1.ConfigMap{},           // OTel Collector config
		&corev1.Secret{},              // TLS certificates and credentials
		&corev1.Service{},             // Collector service endpoints
		&corev1.ServiceAccount{},      // Identity for k8s API access
		&rbacv1.ClusterRole{},         // Permissions for k8s metadata collection
		&rbacv1.ClusterRoleBinding{},  // Binds ClusterRole to ServiceAccount
		&rbacv1.Role{},                // Namespace-scoped permissions for leader election
		&rbacv1.RoleBinding{},         // Binds Role to ServiceAccount
		&networkingv1.NetworkPolicy{}, // Network access control
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(mgr.GetConfig())
	if err != nil {
		return fmt.Errorf("failed to create discovery client: %w", err)
	}

	isIstioActive := istiostatus.NewChecker(discoveryClient).IsIstioActive(context.Background())

	if isIstioActive {
		ownedResourceTypesToWatch = append(ownedResourceTypesToWatch, &istiosecurityclientv1.PeerAuthentication{}) // Istio mTLS policy
	}

	for _, resource := range ownedResourceTypesToWatch {
		b = b.Watches(
			resource,
			handler.EnqueueRequestsFromMapFunc(r.mapOwnedResourceToConfigMap),
			ctrlbuilder.WithPredicates(predicateutils.OwnedResourceChanged()),
		)
	}

	// Watch Telemetry CR
	b.Watches(
		&operatorv1beta1.Telemetry{},
		handler.EnqueueRequestsFromMapFunc(r.mapTelemetryToConfigMap),
		ctrlbuilder.WithPredicates(predicateutils.CreateOrUpdateOrDelete()),
	)

	// Watch Istio resources if present
	if isIstioPresent(mgr.GetClient()) {
		b = b.Watches(
			&istiosecurityclientv1.PeerAuthentication{}, // Istio mTLS policy
			handler.EnqueueRequestsFromMapFunc(r.mapOwnedResourceToConfigMap),
			ctrlbuilder.WithPredicates(predicateutils.OwnedResourceChanged()),
		)
		b = b.Watches(
			&istionetworkingclientv1.DestinationRule{}, // Istio traffic routing rules
			handler.EnqueueRequestsFromMapFunc(r.mapOwnedResourceToConfigMap),
			ctrlbuilder.WithPredicates(predicateutils.OwnedResourceChanged()),
		)
	}

	return b.Complete(r)
}

// mapTelemetryToConfigMap creates a reconcile request for the OTLP Gateway Pipelines Sync ConfigMap when Telemetry CR changes.
func (r *OTLPGatewayController) mapTelemetryToConfigMap(ctx context.Context, object client.Object) []reconcile.Request {
	_, ok := object.(*operatorv1beta1.Telemetry)
	if !ok {
		logf.FromContext(ctx).V(1).Error(nil, "Unexpected type: expected Telemetry")
		return nil
	}

	namespace := object.GetNamespace()

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      names.OTLPGatewayPipelinesSyncConfigMap,
				Namespace: namespace,
			},
		},
	}
}

// mapOwnedResourceToConfigMap maps owned resource changes to reconcile requests for the ConfigMap.
func (r *OTLPGatewayController) mapOwnedResourceToConfigMap(ctx context.Context, object client.Object) []reconcile.Request {
	namespace := object.GetNamespace()

	logf.FromContext(ctx).V(1).Info("owned resource changed, triggering OTLP gateway reconciliation", "resource", object.GetName())

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      names.OTLPGatewayPipelinesSyncConfigMap,
				Namespace: namespace,
			},
		},
	}
}

// isIstioPresent checks if Istio CRDs are installed in the cluster.
func isIstioPresent(c client.Client) bool {
	// Try to list PeerAuthentication resources - if it fails, Istio is not present
	var peerAuths istiosecurityclientv1.PeerAuthenticationList
	return c.List(context.Background(), &peerAuths) == nil
}
