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

package otlpgateway

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/tracegateway"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
	telemetryutils "github.com/kyma-project/telemetry-manager/internal/utils/telemetry"
)

// Reconciler reconciles the OTLP Gateway DaemonSet based on pipeline references in the coordination ConfigMap.
type Reconciler struct {
	client.Client

	globals config.Global

	// Dependencies
	gatewayApplierDeleter GatewayApplierDeleter
	traceConfigBuilder    TraceGatewayConfigBuilder
	gatewayProber         Prober
	istioStatusChecker    IstioStatusChecker
	errToMsgConverter     ErrorToMessageConverter
}

// Option configures the Reconciler during initialization.
type Option func(*Reconciler)

// WithGlobals sets the global configuration.
func WithGlobals(globals config.Global) Option {
	return func(r *Reconciler) {
		r.globals = globals
	}
}

// WithGatewayApplierDeleter sets the gateway applier/deleter.
func WithGatewayApplierDeleter(gad GatewayApplierDeleter) Option {
	return func(r *Reconciler) {
		r.gatewayApplierDeleter = gad
	}
}

// WithTraceConfigBuilder sets the trace configuration builder.
func WithTraceConfigBuilder(builder TraceGatewayConfigBuilder) Option {
	return func(r *Reconciler) {
		r.traceConfigBuilder = builder
	}
}

// WithGatewayProber sets the gateway prober.
func WithGatewayProber(prober Prober) Option {
	return func(r *Reconciler) {
		r.gatewayProber = prober
	}
}

// WithIstioStatusChecker sets the Istio status checker.
func WithIstioStatusChecker(checker IstioStatusChecker) Option {
	return func(r *Reconciler) {
		r.istioStatusChecker = checker
	}
}

// WithErrorToMessageConverter sets the error-to-message converter.
func WithErrorToMessageConverter(converter ErrorToMessageConverter) Option {
	return func(r *Reconciler) {
		r.errToMsgConverter = converter
	}
}

// NewReconciler creates a new OTLP Gateway Reconciler with the given options.
func NewReconciler(c client.Client, opts ...Option) *Reconciler {
	r := &Reconciler{
		Client: c,
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Globals returns a pointer to the global configuration.
func (r *Reconciler) Globals() *config.Global {
	return &r.globals
}

// Reconcile reconciles the OTLP Gateway DaemonSet based on the coordination ConfigMap.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logf.FromContext(ctx).V(1).Info("Reconciling OTLP Gateway")

	// Ensure ConfigMap exists
	if err := r.ensureConfigMapExists(ctx); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile the gateway
	if err := r.doReconcile(ctx); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// ensureConfigMapExists creates the coordination ConfigMap if it doesn't exist.
func (r *Reconciler) ensureConfigMapExists(ctx context.Context) error {
	var cm corev1.ConfigMap
	err := r.Get(ctx, types.NamespacedName{
		Name:      otelcollector.OTLPGatewayConfigMapName,
		Namespace: r.globals.TargetNamespace(),
	}, &cm)

	if err == nil {
		return nil // Already exists
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get OTLP Gateway ConfigMap: %w", err)
	}

	// Create initial ConfigMap
	cm = corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      otelcollector.OTLPGatewayConfigMapName,
			Namespace: r.globals.TargetNamespace(),
		},
		Data: map[string]string{
			otelcollector.ConfigMapDataKey: "TracePipeline: []\n",
		},
	}

	if err := r.Create(ctx, &cm); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create OTLP Gateway ConfigMap: %w", err)
	}

	return nil
}

// doReconcile performs the main reconciliation logic.
func (r *Reconciler) doReconcile(ctx context.Context) error {
	log := logf.FromContext(ctx)

	// Read coordination ConfigMap
	config, err := otelcollector.ReadOTLPGatewayConfig(ctx, r.Client, r.globals.TargetNamespace())
	if err != nil {
		return fmt.Errorf("failed to read OTLP Gateway ConfigMap: %w", err)
	}

	// Fetch TracePipeline CRs
	tracePipelines, err := r.fetchTracePipelines(ctx, config.TracePipeline)
	if err != nil {
		return fmt.Errorf("failed to fetch TracePipeline CRs: %w", err)
	}

	// Extract pipeline names for status updates
	pipelineNames := make([]string, 0, len(tracePipelines))
	for _, pipeline := range tracePipelines {
		pipelineNames = append(pipelineNames, pipeline.Name)
	}

	// If no valid pipelines, delete gateway resources
	if len(tracePipelines) == 0 {
		log.V(1).Info("No valid pipelines found, deleting OTLP Gateway resources")

		if err := r.gatewayApplierDeleter.DeleteResources(ctx, r.Client, r.istioStatusChecker.IsIstioActive(ctx)); err != nil {
			return fmt.Errorf("failed to delete gateway resources: %w", err)
		}

		// Update status on all referenced pipelines (from ConfigMap)
		allReferencedNames := make([]string, 0, len(config.TracePipeline))
		for _, ref := range config.TracePipeline {
			allReferencedNames = append(allReferencedNames, ref.Name)
		}
		if err := r.updateGatewayHealthyConditions(ctx, allReferencedNames); err != nil {
			log.Error(err, "Failed to update GatewayHealthy conditions after deletion")
		}

		// Clean up legacy trace gateway Deployment
		if err := r.cleanupLegacyResources(ctx); err != nil {
			log.Error(err, "Failed to cleanup legacy resources")
			// Don't fail reconciliation on cleanup errors
		}

		return nil
	}

	// Build OTel Collector configuration
	collectorConfig, collectorEnvVars, err := r.buildCollectorConfig(ctx, tracePipelines)
	if err != nil {
		return fmt.Errorf("failed to build collector config: %w", err)
	}

	// Marshal config to YAML
	collectorConfigYAML, err := yaml.Marshal(collectorConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal collector config: %w", err)
	}

	// Deploy/update DaemonSet
	isIstioActive := r.istioStatusChecker.IsIstioActive(ctx)
	opts := otelcollector.GatewayApplyOptions{
		CollectorConfigYAML: string(collectorConfigYAML),
		CollectorEnvVars:    collectorEnvVars,
		IstioEnabled:        isIstioActive,
		// Note: Replicas field is ignored for DaemonSet
		ResourceRequirementsMultiplier: len(tracePipelines),
	}

	if err := r.gatewayApplierDeleter.ApplyResources(ctx, r.Client, opts); err != nil {
		return fmt.Errorf("failed to apply gateway resources: %w", err)
	}

	// Clean up legacy trace gateway Deployment
	if err := r.cleanupLegacyResources(ctx); err != nil {
		log.Error(err, "Failed to cleanup legacy resources")
		// Don't fail reconciliation on cleanup errors
	}

	// Update GatewayHealthy status on all TracePipelines
	if err := r.updateGatewayHealthyConditions(ctx, pipelineNames); err != nil {
		log.Error(err, "Failed to update GatewayHealthy conditions")
		// Don't fail reconciliation on status update errors
	}

	return nil
}

// fetchTracePipelines fetches TracePipeline CRs based on references from the ConfigMap.
// It validates that the generation matches and skips stale or missing pipelines.
func (r *Reconciler) fetchTracePipelines(ctx context.Context, refs []otelcollector.PipelineReference) ([]telemetryv1beta1.TracePipeline, error) {
	log := logf.FromContext(ctx)
	pipelines := make([]telemetryv1beta1.TracePipeline, 0, len(refs))

	for _, ref := range refs {
		var pipeline telemetryv1beta1.TracePipeline
		if err := r.Get(ctx, types.NamespacedName{Name: ref.Name}, &pipeline); err != nil {
			if apierrors.IsNotFound(err) {
				log.V(1).Info("TracePipeline not found, skipping", "pipeline", ref.Name)
				continue
			}
			return nil, fmt.Errorf("failed to get TracePipeline %s: %w", ref.Name, err)
		}

		// Skip pipelines being deleted
		if pipeline.DeletionTimestamp != nil {
			log.V(1).Info("TracePipeline is being deleted, skipping", "pipeline", ref.Name)
			continue
		}

		// Check generation matches (skip stale references)
		if pipeline.Generation != ref.Generation {
			log.V(1).Info("TracePipeline generation mismatch, skipping", "pipeline", ref.Name, "configGeneration", ref.Generation, "actualGeneration", pipeline.Generation)
			continue
		}

		pipelines = append(pipelines, pipeline)
	}

	return pipelines, nil
}

// buildCollectorConfig builds the OpenTelemetry Collector configuration from TracePipeline CRs.
func (r *Reconciler) buildCollectorConfig(ctx context.Context, pipelines []telemetryv1beta1.TracePipeline) (*common.Config, common.EnvVars, error) {
	// Gather cluster information
	shootInfo := k8sutils.GetGardenerShootInfo(ctx, r.Client)
	telemetryOptions := telemetryutils.Options{
		SignalType:                common.SignalTypeTrace,
		Client:                    r.Client,
		DefaultTelemetryNamespace: r.globals.DefaultTelemetryNamespace(),
	}
	clusterName := telemetryutils.GetClusterNameFromTelemetry(ctx, telemetryOptions)

	clusterUID, err := k8sutils.GetClusterUID(ctx, r.Client)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get cluster UID: %w", err)
	}

	// Get enrichment configuration
	var enrichments *operatorv1beta1.EnrichmentSpec
	t, err := telemetryutils.GetDefaultTelemetryInstance(ctx, r.Client, r.globals.DefaultTelemetryNamespace())
	if err == nil {
		enrichments = t.Spec.Enrichments
	}

	// Build configuration
	return r.traceConfigBuilder.Build(ctx, pipelines, tracegateway.BuildOptions{
		Cluster: common.ClusterOptions{
			ClusterName:   clusterName,
			ClusterUID:    clusterUID,
			CloudProvider: shootInfo.CloudProvider,
		},
		Enrichments:       enrichments,
		ServiceEnrichment: telemetryutils.GetServiceEnrichmentFromTelemetryOrDefault(ctx, telemetryOptions),
	})
}

// cleanupLegacyResources removes the old trace gateway Deployment from the previous architecture.
func (r *Reconciler) cleanupLegacyResources(ctx context.Context) error {
	log := logf.FromContext(ctx)

	// Delete old trace gateway Deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.TraceGateway,
			Namespace: r.globals.TargetNamespace(),
		},
	}

	if err := r.Delete(ctx, deployment); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete legacy trace gateway Deployment: %w", err)
	}

	if err := client.IgnoreNotFound(r.Delete(ctx, deployment)); err == nil {
		log.Info("Cleaned up legacy trace gateway Deployment")
	}

	return nil
}
