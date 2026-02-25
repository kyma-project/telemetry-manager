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
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpgateway"
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

// WithConfigBuilder sets the OTLP Gateway configuration builder.
func WithConfigBuilder(builder OTLPGatewayConfigBuilder) Option {
	return func(r *Reconciler) {
		r.configBuilder = builder
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
	logf.FromContext(ctx).V(1).Info("reconciling OTLP gateway")

	if err := r.ensureConfigMapExists(ctx); err != nil {
		return ctrl.Result{}, err
	}

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
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get configmap: %w", err)
	}

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
		return fmt.Errorf("failed to create configmap: %w", err)
	}

	return nil
}

// processConfigAndBuildResources handles config building and resource deployment.
func (r *Reconciler) processConfigAndBuildResources(ctx context.Context, tracePipelines []telemetryv1beta1.TracePipeline, logPipelines []telemetryv1beta1.LogPipeline) error {
	collectorConfig, collectorEnvVars, err := r.buildCollectorConfig(ctx, tracePipelines, logPipelines)
	if err != nil {
		return fmt.Errorf("failed to build config: %w", err)
	}

	collectorConfigYAML, err := yaml.Marshal(collectorConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	isIstioActive := r.istioStatusChecker.IsIstioActive(ctx)
	opts := otelcollector.GatewayApplyOptions{
		CollectorConfigYAML:            string(collectorConfigYAML),
		CollectorEnvVars:               collectorEnvVars,
		IstioEnabled:                   isIstioActive,
		ResourceRequirementsMultiplier: len(tracePipelines) + len(logPipelines),
	}

	return r.gatewayApplierDeleter.ApplyResources(ctx, r.Client, opts)
}

// collectAllReferencedNames extracts all pipeline names from ConfigMap references.
func collectAllReferencedNames(config *otelcollector.OTLPGatewayConfigMap) []string {
	allNames := make([]string, 0, len(config.TracePipeline)+len(config.LogPipeline))
	for _, ref := range config.TracePipeline {
		allNames = append(allNames, ref.Name)
	}

	for _, ref := range config.LogPipeline {
		allNames = append(allNames, ref.Name)
	}

	return allNames
}

// doReconcile performs the main reconciliation logic.
func (r *Reconciler) doReconcile(ctx context.Context) error {
	log := logf.FromContext(ctx)

	config, err := otelcollector.ReadOTLPGatewayConfig(ctx, r.Client, r.globals.TargetNamespace())
	if err != nil {
		return fmt.Errorf("failed to read configmap: %w", err)
	}

	tracePipelines, err := r.fetchTracePipelines(ctx, config.TracePipeline)
	if err != nil {
		return fmt.Errorf("failed to fetch trace pipelines: %w", err)
	}

	logPipelines, err := r.fetchLogPipelines(ctx, config.LogPipeline)
	if err != nil {
		return fmt.Errorf("failed to fetch log pipelines: %w", err)
	}

	// Collect all pipeline names for status updates
	tracePipelineNames := make([]string, 0, len(tracePipelines))
	for _, pipeline := range tracePipelines {
		tracePipelineNames = append(tracePipelineNames, pipeline.Name)
	}

	logPipelineNames := make([]string, 0, len(logPipelines))
	for _, pipeline := range logPipelines {
		logPipelineNames = append(logPipelineNames, pipeline.Name)
	}

	// If no valid pipelines of any type, clean up
	if len(tracePipelines) == 0 && len(logPipelines) == 0 {
		log.V(1).Info("no valid pipelines, deleting gateway resources")

		if err := r.gatewayApplierDeleter.DeleteResources(ctx, r.Client, r.istioStatusChecker.IsIstioActive(ctx)); err != nil {
			return fmt.Errorf("failed to delete gateway: %w", err)
		}

		// Update status for all referenced pipelines
		allReferencedNames := collectAllReferencedNames(config)
		if err := r.updateGatewayHealthyConditions(ctx, allReferencedNames); err != nil {
			log.Error(err, "failed to update status after deletion")
		}

		if err := r.cleanupLegacyResources(ctx); err != nil {
			log.Error(err, "failed to cleanup legacy resources")
		}

		return nil
	}

	// Build and apply resources
	if err := r.processConfigAndBuildResources(ctx, tracePipelines, logPipelines); err != nil {
		return err
	}

	if err := r.cleanupLegacyResources(ctx); err != nil {
		log.Error(err, "failed to cleanup legacy resources")
	}

	// Update status for all pipelines
	allPipelineNames := make([]string, 0, len(tracePipelineNames)+len(logPipelineNames))
	allPipelineNames = append(allPipelineNames, tracePipelineNames...)
	allPipelineNames = append(allPipelineNames, logPipelineNames...)

	if err := r.updateGatewayHealthyConditions(ctx, allPipelineNames); err != nil {
		log.Error(err, "failed to update status")
	}

	return nil
}

// fetchTracePipelines fetches TracePipeline CRs from references.
//
//nolint:dupl // Acceptable duplication - generic approach adds complexity without significant benefit
func (r *Reconciler) fetchTracePipelines(ctx context.Context, refs []otelcollector.PipelineReference) ([]telemetryv1beta1.TracePipeline, error) {
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
func (r *Reconciler) fetchLogPipelines(ctx context.Context, refs []otelcollector.PipelineReference) ([]telemetryv1beta1.LogPipeline, error) {
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

// buildCollectorConfig builds OTel Collector configuration from TracePipeline and LogPipeline CRs.
func (r *Reconciler) buildCollectorConfig(ctx context.Context, tracePipelines []telemetryv1beta1.TracePipeline, logPipelines []telemetryv1beta1.LogPipeline) (*common.Config, common.EnvVars, error) {
	shootInfo := k8sutils.GetGardenerShootInfo(ctx, r.Client)
	telemetryOptions := telemetryutils.Options{
		SignalType:                common.SignalTypeTrace,
		Client:                    r.Client,
		DefaultTelemetryNamespace: r.globals.DefaultTelemetryNamespace(),
	}
	clusterName := telemetryutils.GetClusterNameFromTelemetry(ctx, telemetryOptions)

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
		LogPipelines:   logPipelines,
		TracePipelines: tracePipelines,
		Cluster: common.ClusterOptions{
			ClusterName:   clusterName,
			ClusterUID:    clusterUID,
			CloudProvider: shootInfo.CloudProvider,
		},
		Enrichments:       enrichments,
		ServiceEnrichment: telemetryutils.GetServiceEnrichmentFromTelemetryOrDefault(ctx, telemetryOptions),
		ModuleVersion:     r.globals.Version(),
	})
}

// cleanupLegacyResources removes the old trace gateway Deployment.
func (r *Reconciler) cleanupLegacyResources(ctx context.Context) error {
	log := logf.FromContext(ctx)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.TraceGateway,
			Namespace: r.globals.TargetNamespace(),
		},
	}

	if err := r.Delete(ctx, deployment); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete legacy deployment: %w", err)
	}

	if err := client.IgnoreNotFound(r.Delete(ctx, deployment)); err == nil {
		log.Info("cleaned up legacy trace gateway deployment")
	}

	return nil
}
