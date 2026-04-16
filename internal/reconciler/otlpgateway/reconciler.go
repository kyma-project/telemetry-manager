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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpgateway"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/resources/coordinationconfig"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
	telemetryutils "github.com/kyma-project/telemetry-manager/internal/utils/telemetry"
)

// Reconciler reconciles the OTLP Gateway DaemonSet based on pipeline references in the coordination ConfigMap.
type Reconciler struct {
	client.Client

	globals config.Global

	// Dependencies
	gatewayApplierDeleter GatewayApplierDeleter
	configBuilder         OTLPGatewayConfigBuilder
	istioStatusChecker    IstioStatusChecker
	vpaStatusChecker      VpaStatusChecker
	nodeSizeTracker       NodeSizeTracker
	overridesHandler      *overrides.Handler
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

// WithConfigBuilder sets the OTLP Gateway configuration builder.
func WithConfigBuilder(builder OTLPGatewayConfigBuilder) Option {
	return func(r *Reconciler) {
		r.configBuilder = builder
	}
}

// WithIstioStatusChecker sets the Istio status checker.
func WithIstioStatusChecker(checker IstioStatusChecker) Option {
	return func(r *Reconciler) {
		r.istioStatusChecker = checker
	}
}

// WithVpaStatusChecker sets the VPA status checker.
func WithVpaStatusChecker(checker VpaStatusChecker) Option {
	return func(r *Reconciler) {
		r.vpaStatusChecker = checker
	}
}

// WithNodeSizeTracker sets the node size tracker.
func WithNodeSizeTracker(tracker NodeSizeTracker) Option {
	return func(r *Reconciler) {
		r.nodeSizeTracker = tracker
	}
}

// WithOverridesHandler sets the overrides handler.
func WithOverridesHandler(handler *overrides.Handler) Option {
	return func(r *Reconciler) {
		r.overridesHandler = handler
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
	logf.FromContext(ctx).V(1).Info("reconciling OTLP gateway")

	// Load overrides and check if OTLP Gateway is paused
	if r.overridesHandler != nil {
		overrideConfig, err := r.overridesHandler.LoadOverrides(ctx)
		if err != nil {
			return ctrl.Result{}, err
		}

		if overrideConfig.OTLPGateway.Paused {
			logf.FromContext(ctx).V(1).Info("Skipping reconciliation: paused using override config")
			return ctrl.Result{}, nil
		}
	}

	if err := r.doReconcile(ctx); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// processConfigAndBuildResources handles config building and resource deployment.
func (r *Reconciler) processConfigAndBuildResources(ctx context.Context, tracePipelines []telemetryv1beta1.TracePipeline, logPipelines []telemetryv1beta1.LogPipeline, metricPipelines []telemetryv1beta1.MetricPipeline) error {
	collectorConfig, collectorEnvVars, err := r.buildCollectorConfig(ctx, tracePipelines, logPipelines, metricPipelines)
	if err != nil {
		return fmt.Errorf("failed to build config: %w", err)
	}

	collectorConfigYAML, err := yaml.Marshal(collectorConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	isIstioActive, err := r.istioStatusChecker.IsIstioActive(ctx)
	if err != nil {
		return fmt.Errorf("failed to check Istio status: %w", err)
	}

	vpaCRDExists, err := r.vpaStatusChecker.VpaCRDExists(ctx, r.Client)
	if err != nil {
		return fmt.Errorf("failed to check VPA CRD: %w", err)
	}

	vpaEnabled := telemetryutils.IsVpaEnabledInTelemetry(ctx, r.Client, r.globals.DefaultTelemetryNamespace())
	vpaMaxAllowedMemory := r.nodeSizeTracker.VPAMaxAllowedMemory()

	opts := otelcollector.GatewayApplyOptions{
		CollectorConfigYAML:            string(collectorConfigYAML),
		CollectorEnvVars:               collectorEnvVars,
		IstioEnabled:                   isIstioActive,
		ResourceRequirementsMultiplier: len(tracePipelines) + len(logPipelines) + len(metricPipelines),
		VpaCRDExists:                   vpaCRDExists,
		VpaEnabled:                     vpaEnabled,
		VPAMaxAllowedMemory:            vpaMaxAllowedMemory,
	}

	return r.gatewayApplierDeleter.ApplyResources(ctx, r.Client, opts)
}

// doReconcile performs the main reconciliation logic.
func (r *Reconciler) doReconcile(ctx context.Context) error {
	log := logf.FromContext(ctx)

	// Clean up legacy per-signal gateway resources (from pre-OTLP-Gateway architecture)
	if err := r.cleanupLegacyGateways(ctx); err != nil {
		return fmt.Errorf("failed to clean up legacy gateways: %w", err)
	}

	config, err := coordinationconfig.ReadOTLPGatewayConfig(ctx, r.Client, r.globals.TargetNamespace())
	if err != nil {
		return fmt.Errorf("failed to read configmap: %w", err)
	}

	tracePipelines, err := r.fetchTracePipelines(ctx, config.TracePipelineReferences)
	if err != nil {
		return fmt.Errorf("failed to fetch trace pipelines: %w", err)
	}

	logPipelines, err := r.fetchLogPipelines(ctx, config.LogPipelineReferences)
	if err != nil {
		return fmt.Errorf("failed to fetch log pipelines: %w", err)
	}

	metricPipelines, err := r.fetchMetricPipelines(ctx, config.MetricPipelineReferences)
	if err != nil {
		return fmt.Errorf("failed to fetch metric pipelines: %w", err)
	}

	// If no valid pipelines of any type, clean up
	if len(tracePipelines) == 0 && len(logPipelines) == 0 && len(metricPipelines) == 0 {
		log.V(1).Info("no valid pipelines, deleting gateway resources")

		isIstioActive, err := r.istioStatusChecker.IsIstioActive(ctx)
		if err != nil {
			return fmt.Errorf("failed to check Istio status: %w", err)
		}

		vpaCRDExists, err := r.vpaStatusChecker.VpaCRDExists(ctx, r.Client)
		if err != nil {
			return fmt.Errorf("failed to check VPA CRD: %w", err)
		}

		if err := r.gatewayApplierDeleter.DeleteResources(ctx, r.Client, isIstioActive, vpaCRDExists); err != nil {
			return fmt.Errorf("failed to delete gateway: %w", err)
		}

		// Pipeline controllers will detect gateway deletion via DaemonSet watch and update their own status

		return nil
	}

	// Build and apply resources
	if err := r.processConfigAndBuildResources(ctx, tracePipelines, logPipelines, metricPipelines); err != nil {
		return err
	}

	// Pipeline controllers manage their own GatewayHealthy status conditions

	return nil
}

// fetchTracePipelines fetches TracePipeline CRs from references.
//
//nolint:dupl // Acceptable duplication - generic approach adds complexity without significant benefit
func (r *Reconciler) fetchTracePipelines(ctx context.Context, refs []coordinationconfig.PipelineReference) ([]telemetryv1beta1.TracePipeline, error) {
	log := logf.FromContext(ctx)
	pipelines := make([]telemetryv1beta1.TracePipeline, 0, len(refs))

	for _, ref := range refs {
		var pipeline telemetryv1beta1.TracePipeline
		if err := r.Get(ctx, types.NamespacedName{Name: ref.Name}, &pipeline); err != nil {
			if apierrors.IsNotFound(err) {
				log.V(1).Info("pipeline not found, skipping", "pipeline", ref.Name)
				continue
			}

			return nil, fmt.Errorf("failed to get pipeline %s: %w", ref.Name, err)
		}

		if pipeline.DeletionTimestamp != nil {
			log.V(1).Info("pipeline being deleted, skipping", "pipeline", ref.Name)
			continue
		}

		if pipeline.Generation != ref.Generation {
			log.V(1).Info("pipeline generation mismatch, skipping", "pipeline", ref.Name, "configGeneration", ref.Generation, "actualGeneration", pipeline.Generation)
			continue
		}

		pipelines = append(pipelines, pipeline)
	}

	return pipelines, nil
}

// fetchLogPipelines fetches LogPipeline CRs from references.
//
//nolint:dupl // Acceptable duplication - generic approach adds complexity without significant benefit
func (r *Reconciler) fetchLogPipelines(ctx context.Context, refs []coordinationconfig.PipelineReference) ([]telemetryv1beta1.LogPipeline, error) {
	log := logf.FromContext(ctx)
	pipelines := make([]telemetryv1beta1.LogPipeline, 0, len(refs))

	for _, ref := range refs {
		var pipeline telemetryv1beta1.LogPipeline
		if err := r.Get(ctx, types.NamespacedName{Name: ref.Name}, &pipeline); err != nil {
			if apierrors.IsNotFound(err) {
				log.V(1).Info("log pipeline not found, skipping", "pipeline", ref.Name)
				continue
			}

			return nil, fmt.Errorf("failed to get log pipeline %s: %w", ref.Name, err)
		}

		if pipeline.DeletionTimestamp != nil {
			log.V(1).Info("log pipeline being deleted, skipping", "pipeline", ref.Name)
			continue
		}

		if pipeline.Generation != ref.Generation {
			log.V(1).Info("log pipeline generation mismatch, skipping", "pipeline", ref.Name, "configGeneration", ref.Generation, "actualGeneration", pipeline.Generation)
			continue
		}

		pipelines = append(pipelines, pipeline)
	}

	return pipelines, nil
}

// fetchMetricPipelines fetches MetricPipeline CRs from references.
//
//nolint:dupl // Acceptable duplication - generic approach adds complexity without significant benefit
func (r *Reconciler) fetchMetricPipelines(ctx context.Context, refs []coordinationconfig.PipelineReference) ([]telemetryv1beta1.MetricPipeline, error) {
	log := logf.FromContext(ctx)
	pipelines := make([]telemetryv1beta1.MetricPipeline, 0, len(refs))

	for _, ref := range refs {
		var pipeline telemetryv1beta1.MetricPipeline
		if err := r.Get(ctx, types.NamespacedName{Name: ref.Name}, &pipeline); err != nil {
			if apierrors.IsNotFound(err) {
				log.V(1).Info("metric pipeline not found, skipping", "pipeline", ref.Name)
				continue
			}

			return nil, fmt.Errorf("failed to get metric pipeline %s: %w", ref.Name, err)
		}

		if pipeline.DeletionTimestamp != nil {
			log.V(1).Info("metric pipeline being deleted, skipping", "pipeline", ref.Name)
			continue
		}

		if pipeline.Generation != ref.Generation {
			log.V(1).Info("metric pipeline generation mismatch, skipping", "pipeline", ref.Name, "configGeneration", ref.Generation, "actualGeneration", pipeline.Generation)
			continue
		}

		pipelines = append(pipelines, pipeline)
	}

	return pipelines, nil
}

// buildCollectorConfig builds OTel Collector configuration from TracePipeline, LogPipeline, and MetricPipeline CRs.
func (r *Reconciler) buildCollectorConfig(ctx context.Context, tracePipelines []telemetryv1beta1.TracePipeline, logPipelines []telemetryv1beta1.LogPipeline, metricPipelines []telemetryv1beta1.MetricPipeline) (*common.Config, common.EnvVars, error) {
	shootInfo := k8sutils.GetGardenerShootInfo(ctx, r.Client)
	clusterName := telemetryutils.GetClusterNameFromTelemetry(ctx, r.Client, r.globals.DefaultTelemetryNamespace())

	clusterUID, err := k8sutils.GetClusterUID(ctx, r.Client)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get cluster uid: %w", err)
	}

	var enrichments *operatorv1beta1.EnrichmentSpec

	t, err := telemetryutils.GetDefaultTelemetryInstance(ctx, r.Client, r.globals.DefaultTelemetryNamespace())
	if err == nil {
		enrichments = t.Spec.Enrichments
	}

	return r.configBuilder.Build(ctx, otlpgateway.BuildOptions{
		LogPipelines:    logPipelines,
		TracePipelines:  tracePipelines,
		MetricPipelines: metricPipelines,
		Cluster: common.ClusterOptions{
			ClusterName:   clusterName,
			ClusterUID:    clusterUID,
			CloudProvider: shootInfo.CloudProvider,
		},
		Enrichments:       enrichments,
		ServiceEnrichment: telemetryutils.GetServiceEnrichmentFromTelemetryOrDefault(ctx, r.Client, r.globals.DefaultTelemetryNamespace()),
		ModuleVersion:     r.globals.Version(),
		GatewayNamespace:  r.globals.TargetNamespace(),
	})
}

// TODO: Remove after first roll-out
// cleanupLegacyGateways removes leftover resources from the old per-signal gateway Deployments
// (telemetry-trace-gateway, telemetry-metric-gateway, telemetry-log-gateway) that existed before
// the centralized OTLP Gateway architecture. This is idempotent and safe on clusters where
// old resources don't exist.
func (r *Reconciler) cleanupLegacyGateways(ctx context.Context) error {
	isIstioActive, err := r.istioStatusChecker.IsIstioActive(ctx)
	if err != nil {
		return fmt.Errorf("failed to check Istio status: %w", err)
	}

	vpaCRDExists, err := r.vpaStatusChecker.VpaCRDExists(ctx, r.Client)
	if err != nil {
		return fmt.Errorf("failed to check VPA CRD: %w", err)
	}

	ns := r.globals.TargetNamespace()
	for _, gatewayName := range []string{names.TraceGateway, names.MetricGateway, names.LogGateway} {
		if err := otelcollector.DeleteLegacyGatewayResources(ctx, r.Client, ns, gatewayName, isIstioActive, vpaCRDExists); err != nil {
			return fmt.Errorf("failed to delete legacy gateway %s: %w", gatewayName, err)
		}
	}

	return nil
}
