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

	istionetworkingv1 "istio.io/client-go/pkg/apis/networking/v1"
	istiosecurityv1 "istio.io/client-go/pkg/apis/security/v1"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/istiostatus"
	otlpgatewayconfig "github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpgateway"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/otlpgateway"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	predicateutils "github.com/kyma-project/telemetry-manager/internal/utils/predicate"
	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

// OTLPGatewayController reconciles the OTLP Gateway DaemonSet based on pipeline references
type OTLPGatewayController struct {
	client.Client

	reconcileTriggerChan <-chan event.GenericEvent
	reconciler           *otlpgateway.Reconciler
}

type OTLPGatewayControllerConfig struct {
	config.Global

	RestConfig                    *rest.Config
	OTelCollectorImage            string
	OTLPGatewayPriorityClassName string
}

func NewOTLPGatewayController(config OTLPGatewayControllerConfig, client client.Client, reconcileTriggerChan <-chan event.GenericEvent) (*OTLPGatewayController, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config.RestConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}

	reconciler := otlpgateway.NewReconciler(
		client,
		otlpgateway.WithGlobals(config.Global),
		otlpgateway.WithGatewayApplierDeleter(
			otelcollector.NewOTLPGatewayApplierDeleter(
				config.Global,
				config.OTelCollectorImage,
				config.OTLPGatewayPriorityClassName,
			),
		),
		otlpgateway.WithConfigBuilder(&otlpgatewayconfig.Builder{Reader: client}),
		otlpgateway.WithGatewayProber(&workloadstatus.DaemonSetProber{Client: client}),
		otlpgateway.WithIstioStatusChecker(istiostatus.NewChecker(discoveryClient)),
		otlpgateway.WithErrorToMessageConverter(&conditions.ErrorToMessageConverter{}),
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
	// Primary watch: OTLP Gateway ConfigMap
	b := ctrl.NewControllerManagedBy(mgr).For(&corev1.ConfigMap{},
		ctrlbuilder.WithPredicates(predicateutils.CreateOrUpdateOrDelete()),
	)

	// Filter to only watch the OTLP Gateway ConfigMap specifically
	// Note: We watch all ConfigMaps and filter in the reconcile loop to avoid complexity

	// Secondary watch: TracePipeline CRs (to detect spec changes)
	b = b.Watches(
		&telemetryv1beta1.TracePipeline{},
		handler.EnqueueRequestsFromMapFunc(r.mapPipelineToConfigMap),
		ctrlbuilder.WithPredicates(predicateutils.CreateOrUpdateOrDelete()),
	)

	// Watch reconcile trigger channel
	b.WatchesRawSource(
		source.Channel(r.reconcileTriggerChan, &handler.EnqueueRequestForObject{}),
	)

	// Watch owned resources (DaemonSet and associated resources)
	ownedResourceTypesToWatch := []client.Object{
		&appsv1.DaemonSet{},
		&corev1.ConfigMap{}, // OTel Collector config
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
			handler.EnqueueRequestsFromMapFunc(r.mapOwnedResourceToConfigMap),
			ctrlbuilder.WithPredicates(predicateutils.OwnedResourceChanged()),
		)
	}

	// Watch Istio resources if present
	if isIstioPresent(mgr.GetClient()) {
		b = b.Watches(
			&istiosecurityv1.PeerAuthentication{},
			handler.EnqueueRequestsFromMapFunc(r.mapOwnedResourceToConfigMap),
			ctrlbuilder.WithPredicates(predicateutils.OwnedResourceChanged()),
		)
		b = b.Watches(
			&istionetworkingv1.DestinationRule{},
			handler.EnqueueRequestsFromMapFunc(r.mapOwnedResourceToConfigMap),
			ctrlbuilder.WithPredicates(predicateutils.OwnedResourceChanged()),
		)
	}

	return b.Complete(r)
}

// mapPipelineToConfigMap maps TracePipeline changes to reconcile requests for the ConfigMap.
func (r *OTLPGatewayController) mapPipelineToConfigMap(ctx context.Context, object client.Object) []reconcile.Request {
	pipeline, ok := object.(*telemetryv1beta1.TracePipeline)
	if !ok {
		logf.FromContext(ctx).V(1).Error(nil, "unexpected type: expected TracePipeline")
		return nil
	}

	namespace := "kyma-system"
	if r.reconciler != nil {
		namespace = r.reconciler.Globals().TargetNamespace()
	}

	logf.FromContext(ctx).V(1).Info("pipeline changed, triggering OTLP gateway reconciliation", "pipeline", pipeline.Name)

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      otelcollector.OTLPGatewayConfigMapName,
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
				Name:      otelcollector.OTLPGatewayConfigMapName,
				Namespace: namespace,
			},
		},
	}
}

// isIstioPresent checks if Istio CRDs are installed in the cluster.
func isIstioPresent(c client.Client) bool {
	// Try to list PeerAuthentication resources - if it fails, Istio is not present
	var peerAuths istiosecurityv1.PeerAuthenticationList
	return c.List(context.Background(), &peerAuths) == nil
}
