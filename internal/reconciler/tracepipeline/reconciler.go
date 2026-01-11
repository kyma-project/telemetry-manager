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

package tracepipeline

import (
	"context"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
	"github.com/kyma-project/telemetry-manager/internal/metrics"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/tracegateway"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
	sharedtypesutils "github.com/kyma-project/telemetry-manager/internal/utils/sharedtypes"
	telemetryutils "github.com/kyma-project/telemetry-manager/internal/utils/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

// defaultReplicaCount is the default number of trace gateway replicas when no custom scaling configuration is provided.
const defaultReplicaCount int32 = 2

type Reconciler struct {
	client.Client

	config config.Global

	// Dependencies
	flowHealthProber      FlowHealthProber
	gatewayApplierDeleter GatewayApplierDeleter
	gatewayConfigBuilder  GatewayConfigBuilder
	gatewayProber         commonstatus.Prober
	istioStatusChecker    IstioStatusChecker
	overridesHandler      OverridesHandler
	pipelineLock          PipelineLock
	pipelineSync          PipelineSyncer
	pipelineValidator     *Validator
	errToMsgConverter     commonstatus.ErrorToMessageConverter
}

// Option configures the Reconciler during initialization.
type Option func(*Reconciler)

// WithGlobal sets the global configuration for the Reconciler.
func WithGlobal(cfg config.Global) Option {
	return func(r *Reconciler) {
		r.config = cfg
	}
}

// WithFlowHealthProber sets the flow health prober for the Reconciler.
func WithFlowHealthProber(prober FlowHealthProber) Option {
	return func(r *Reconciler) {
		r.flowHealthProber = prober
	}
}

// WithGatewayApplierDeleter sets the gateway applier/deleter for the Reconciler.
func WithGatewayApplierDeleter(applierDeleter GatewayApplierDeleter) Option {
	return func(r *Reconciler) {
		r.gatewayApplierDeleter = applierDeleter
	}
}

// WithGatewayConfigBuilder sets the gateway configuration builder for the Reconciler.
func WithGatewayConfigBuilder(builder GatewayConfigBuilder) Option {
	return func(r *Reconciler) {
		r.gatewayConfigBuilder = builder
	}
}

// WithGatewayProber sets the gateway prober for the Reconciler.
func WithGatewayProber(prober commonstatus.Prober) Option {
	return func(r *Reconciler) {
		r.gatewayProber = prober
	}
}

// WithIstioStatusChecker sets the Istio status checker for the Reconciler.
func WithIstioStatusChecker(checker IstioStatusChecker) Option {
	return func(r *Reconciler) {
		r.istioStatusChecker = checker
	}
}

// WithOverridesHandler sets the overrides handler for the Reconciler.
func WithOverridesHandler(handler OverridesHandler) Option {
	return func(r *Reconciler) {
		r.overridesHandler = handler
	}
}

// WithPipelineLock sets the pipeline lock for the Reconciler.
func WithPipelineLock(lock PipelineLock) Option {
	return func(r *Reconciler) {
		r.pipelineLock = lock
	}
}

// WithPipelineSyncer sets the pipeline syncer for the Reconciler.
func WithPipelineSyncer(syncer PipelineSyncer) Option {
	return func(r *Reconciler) {
		r.pipelineSync = syncer
	}
}

// WithPipelineValidator sets the pipeline validator for the Reconciler.
func WithPipelineValidator(validator *Validator) Option {
	return func(r *Reconciler) {
		r.pipelineValidator = validator
	}
}

// WithErrorToMessageConverter sets the error to message converter for the Reconciler.
func WithErrorToMessageConverter(converter commonstatus.ErrorToMessageConverter) Option {
	return func(r *Reconciler) {
		r.errToMsgConverter = converter
	}
}

// WithClient sets the Kubernetes client for the Reconciler.
func WithClient(client client.Client) Option {
	return func(r *Reconciler) {
		r.Client = client
	}
}

// New creates a new Reconciler with the provided options.
func New(opts ...Option) *Reconciler {
	r := &Reconciler{}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Reconcile reconciles a TracePipeline resource by ensuring the trace gateway is properly configured and deployed.
// It handles pipeline locking, validation, and status updates.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logf.FromContext(ctx).V(1).Info("Reconciling")

	overrideConfig, err := r.overridesHandler.LoadOverrides(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	if overrideConfig.Tracing.Paused {
		logf.FromContext(ctx).V(1).Info("Skipping reconciliation: paused using override config")
		return ctrl.Result{}, nil
	}

	var tracePipeline telemetryv1beta1.TracePipeline
	if err := r.Get(ctx, req.NamespacedName, &tracePipeline); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.pipelineSync.TryAcquireLock(ctx, &tracePipeline); err != nil {
		if errors.Is(err, resourcelock.ErrMaxPipelinesExceeded) {
			logf.FromContext(ctx).V(1).Error(err, "Could not register pipeline")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	err = r.doReconcile(ctx, &tracePipeline)
	if statusErr := r.updateStatus(ctx, tracePipeline.Name); statusErr != nil {
		if err != nil {
			err = fmt.Errorf("failed while updating status: %w: %w", statusErr, err)
		} else {
			err = fmt.Errorf("failed to update status: %w", statusErr)
		}
	}

	return ctrl.Result{}, err
}

// doReconcile performs the main reconciliation logic for a TracePipeline.
// It lists all pipelines, determines which are reconcilable, and either deploys or deletes the trace gateway accordingly.
func (r *Reconciler) doReconcile(ctx context.Context, pipeline *telemetryv1beta1.TracePipeline) error {
	if err := r.pipelineLock.TryAcquireLock(ctx, pipeline); err != nil {
		if errors.Is(err, resourcelock.ErrMaxPipelinesExceeded) {
			logf.FromContext(ctx).V(1).Info("Skipping reconciliation: maximum pipeline count limit exceeded")
			return nil
		}

		return err
	}

	var allPipelinesList telemetryv1beta1.TracePipelineList
	if err := r.List(ctx, &allPipelinesList); err != nil {
		return fmt.Errorf("failed to list trace pipelines: %w", err)
	}

	reconcilablePipelines, err := r.getReconcilablePipelines(ctx, allPipelinesList.Items)
	if err != nil {
		return fmt.Errorf("failed to fetch deployable trace pipelines: %w", err)
	}

	r.trackOTTLFeaturesUsage(reconcilablePipelines)

	if len(reconcilablePipelines) == 0 {
		logf.FromContext(ctx).V(1).Info("cleaning up trace pipeline resources: all trace pipelines are non-reconcilable")

		if err = r.gatewayApplierDeleter.DeleteResources(ctx, r.Client, r.istioStatusChecker.IsIstioActive(ctx)); err != nil {
			return fmt.Errorf("failed to delete gateway resources: %w", err)
		}

		return nil
	}

	if err = r.reconcileTraceGateway(ctx, pipeline, reconcilablePipelines); err != nil {
		return fmt.Errorf("failed to reconcile trace gateway: %w", err)
	}

	return nil
}

// getReconcilablePipelines returns the list of trace pipelines that are ready to be rendered into the otel collector configuration. A pipeline is deployable if it is not being deleted, all secret references exist, and is not above the pipeline limit.
func (r *Reconciler) getReconcilablePipelines(ctx context.Context, allPipelines []telemetryv1beta1.TracePipeline) ([]telemetryv1beta1.TracePipeline, error) {
	var reconcilablePipelines []telemetryv1beta1.TracePipeline

	for i := range allPipelines {
		isReconcilable, err := r.isReconcilable(ctx, &allPipelines[i])
		if err != nil {
			return nil, err
		}

		if isReconcilable {
			reconcilablePipelines = append(reconcilablePipelines, allPipelines[i])
		}
	}

	return reconcilablePipelines, nil
}

// isReconcilable determines whether a TracePipeline is ready to be reconciled.
// A pipeline is reconcilable if it is not being deleted, passes validation, and has valid certificate references.
// Pipelines with certificates about to expire are still considered reconcilable.
func (r *Reconciler) isReconcilable(ctx context.Context, pipeline *telemetryv1beta1.TracePipeline) (bool, error) {
	if !pipeline.GetDeletionTimestamp().IsZero() {
		return false, nil
	}

	err := r.pipelineValidator.validate(ctx, pipeline)

	// Pipeline with a certificate that is about to expire is still considered reconcilable
	if err == nil || tlscert.IsCertAboutToExpireError(err) {
		return true, nil
	}

	// Remaining errors imply that the pipeline is not reconcilable
	// In case that one of the requests to the Kubernetes API server failed, then the pipeline is also considered non-reconcilable and the error is returned to trigger a requeue
	var APIRequestFailed *errortypes.APIRequestFailedError
	if errors.As(err, &APIRequestFailed) {
		return false, APIRequestFailed.Err
	}

	return false, nil
}

// reconcileTraceGateway reconciles the trace gateway by building and applying the OpenTelemetry Collector configuration.
// It gathers cluster information, builds the collector configuration from all reconcilable pipelines, and applies the gateway resources.
func (r *Reconciler) reconcileTraceGateway(ctx context.Context, pipeline *telemetryv1beta1.TracePipeline, allPipelines []telemetryv1beta1.TracePipeline) error {
	shootInfo := k8sutils.GetGardenerShootInfo(ctx, r.Client)
	clusterName := r.getClusterNameFromTelemetry(ctx, shootInfo.ClusterName)

	clusterUID, err := r.getK8sClusterUID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get kube-system namespace for cluster UID: %w", err)
	}

	var enrichments *operatorv1beta1.EnrichmentSpec

	t, err := telemetryutils.GetDefaultTelemetryInstance(ctx, r.Client, r.config.DefaultTelemetryNamespace())
	if err == nil {
		enrichments = t.Spec.Enrichments
	}

	collectorConfig, collectorEnvVars, err := r.gatewayConfigBuilder.Build(ctx, allPipelines, tracegateway.BuildOptions{
		Cluster: common.ClusterOptions{
			ClusterName:   clusterName,
			ClusterUID:    clusterUID,
			CloudProvider: shootInfo.CloudProvider,
		},
		Enrichments: enrichments,
	})
	if err != nil {
		return fmt.Errorf("failed to create collector config: %w", err)
	}

	collectorConfigYAML, err := yaml.Marshal(collectorConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal collector config: %w", err)
	}

	isIstioActive := r.istioStatusChecker.IsIstioActive(ctx)

	opts := otelcollector.GatewayApplyOptions{
		CollectorConfigYAML:            string(collectorConfigYAML),
		CollectorEnvVars:               collectorEnvVars,
		IstioEnabled:                   isIstioActive,
		Replicas:                       r.getReplicaCountFromTelemetry(ctx),
		ResourceRequirementsMultiplier: len(allPipelines),
	}

	if err := r.gatewayApplierDeleter.ApplyResources(
		ctx,
		k8sutils.NewOwnerReferenceSetter(r.Client, pipeline),
		opts,
	); err != nil {
		return fmt.Errorf("failed to apply gateway resources: %w", err)
	}

	return nil
}

// getReplicaCountFromTelemetry retrieves the desired number of trace gateway replicas from the Telemetry CR.
// It returns the configured replica count if static scaling is configured, otherwise returns the default replica count.
func (r *Reconciler) getReplicaCountFromTelemetry(ctx context.Context) int32 {
	telemetry, err := telemetryutils.GetDefaultTelemetryInstance(ctx, r.Client, r.config.DefaultTelemetryNamespace())
	if err != nil {
		logf.FromContext(ctx).V(1).Error(err, "Failed to get telemetry: using default scaling")
		return defaultReplicaCount
	}

	if telemetry.Spec.Trace != nil &&
		telemetry.Spec.Trace.Gateway.Scaling.Type == operatorv1beta1.StaticScalingStrategyType &&
		telemetry.Spec.Trace.Gateway.Scaling.Static != nil &&
		telemetry.Spec.Trace.Gateway.Scaling.Static.Replicas > 0 {
		return telemetry.Spec.Trace.Gateway.Scaling.Static.Replicas
	}

	return defaultReplicaCount
}

// getClusterNameFromTelemetry retrieves the cluster name from the Telemetry CR enrichment configuration.
// If no custom cluster name is configured, it returns the provided default name.
func (r *Reconciler) getClusterNameFromTelemetry(ctx context.Context, defaultName string) string {
	telemetry, err := telemetryutils.GetDefaultTelemetryInstance(ctx, r.Client, r.config.DefaultTelemetryNamespace())
	if err != nil {
		logf.FromContext(ctx).V(1).Error(err, "Failed to get telemetry: using default shoot name as cluster name")
		return defaultName
	}

	if telemetry.Spec.Enrichments != nil &&
		telemetry.Spec.Enrichments.Cluster != nil &&
		telemetry.Spec.Enrichments.Cluster.Name != "" {
		return telemetry.Spec.Enrichments.Cluster.Name
	}

	return defaultName
}

// getK8sClusterUID retrieves the unique identifier of the Kubernetes cluster by fetching the UID of the kube-system namespace.
func (r *Reconciler) getK8sClusterUID(ctx context.Context) (string, error) {
	var kubeSystem corev1.Namespace

	kubeSystemNs := types.NamespacedName{
		Name: "kube-system",
	}

	err := r.Get(ctx, kubeSystemNs, &kubeSystem)
	if err != nil {
		return "", err
	}

	return string(kubeSystem.UID), nil
}

func (r *Reconciler) trackOTTLFeaturesUsage(pipelines []telemetryv1beta1.TracePipeline) {
	for i := range pipelines {
		// General features
		if sharedtypesutils.IsTransformDefined(pipelines[i].Spec.Transforms) {
			metrics.RecordTracePipelineFeatureUsage(metrics.FeatureTransform, pipelines[i].Name)
		}

		if sharedtypesutils.IsFilterDefined(pipelines[i].Spec.Filters) {
			metrics.RecordTracePipelineFeatureUsage(metrics.FeatureFilter, pipelines[i].Name)
		}
	}
}
