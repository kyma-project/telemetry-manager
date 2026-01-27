package otel

import (
	"context"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
	"github.com/kyma-project/telemetry-manager/internal/metrics"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/logagent"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/loggateway"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
	sharedtypesutils "github.com/kyma-project/telemetry-manager/internal/utils/sharedtypes"
	telemetryutils "github.com/kyma-project/telemetry-manager/internal/utils/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

const defaultReplicaCount int32 = 2

type Reconciler struct {
	client.Client

	globals config.Global

	// Dependencies
	gatewayFlowHealthProber GatewayFlowHealthProber
	agentFlowHealthProber   AgentFlowHealthProber
	agentConfigBuilder      AgentConfigBuilder
	agentProber             Prober
	agentApplierDeleter     AgentApplierDeleter
	gatewayApplierDeleter   GatewayApplierDeleter
	gatewayConfigBuilder    GatewayConfigBuilder
	gatewayProber           Prober
	istioStatusChecker      IstioStatusChecker
	pipelineLock            PipelineLock
	pipelineValidator       *Validator
	errToMessageConverter   ErrorToMessageConverter
}

// Option is a functional option for configuring a Reconciler.
type Option func(*Reconciler)

// WithGlobals sets the global configuration.
func WithGlobals(globals config.Global) Option {
	return func(r *Reconciler) {
		r.globals = globals
	}
}

// WithGatewayFlowHealthProber sets the gateway flow health prober.
func WithGatewayFlowHealthProber(prober GatewayFlowHealthProber) Option {
	return func(r *Reconciler) {
		r.gatewayFlowHealthProber = prober
	}
}

// WithAgentFlowHealthProber sets the agent flow health prober.
func WithAgentFlowHealthProber(prober AgentFlowHealthProber) Option {
	return func(r *Reconciler) {
		r.agentFlowHealthProber = prober
	}
}

// WithAgentConfigBuilder sets the agent config builder.
func WithAgentConfigBuilder(builder AgentConfigBuilder) Option {
	return func(r *Reconciler) {
		r.agentConfigBuilder = builder
	}
}

// WithAgentProber sets the agent prober.
func WithAgentProber(prober Prober) Option {
	return func(r *Reconciler) {
		r.agentProber = prober
	}
}

// WithAgentApplierDeleter sets the agent applier/deleter.
func WithAgentApplierDeleter(applierDeleter AgentApplierDeleter) Option {
	return func(r *Reconciler) {
		r.agentApplierDeleter = applierDeleter
	}
}

// WithGatewayApplierDeleter sets the gateway applier/deleter.
func WithGatewayApplierDeleter(applierDeleter GatewayApplierDeleter) Option {
	return func(r *Reconciler) {
		r.gatewayApplierDeleter = applierDeleter
	}
}

// WithGatewayConfigBuilder sets the gateway config builder.
func WithGatewayConfigBuilder(builder GatewayConfigBuilder) Option {
	return func(r *Reconciler) {
		r.gatewayConfigBuilder = builder
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

// WithPipelineLock sets the pipeline lock.
func WithPipelineLock(lock PipelineLock) Option {
	return func(r *Reconciler) {
		r.pipelineLock = lock
	}
}

// WithPipelineValidator sets the pipeline validator.
func WithPipelineValidator(validator *Validator) Option {
	return func(r *Reconciler) {
		r.pipelineValidator = validator
	}
}

// WithErrorToMessageConverter sets the error to message converter.
func WithErrorToMessageConverter(converter ErrorToMessageConverter) Option {
	return func(r *Reconciler) {
		r.errToMessageConverter = converter
	}
}

// WithClient sets the Kubernetes client.
func WithClient(client client.Client) Option {
	return func(r *Reconciler) {
		r.Client = client
	}
}

// New creates a new Reconciler with the provided client and functional options.
// All dependencies must be provided via functional options.
func New(opts ...Option) *Reconciler {
	r := &Reconciler{}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

func (r *Reconciler) Reconcile(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline) error {
	logf.FromContext(ctx).V(1).Info("Reconciling LogPipeline")

	err := r.doReconcile(ctx, pipeline)
	if statusErr := r.updateStatus(ctx, pipeline.Name); statusErr != nil {
		if err != nil {
			err = fmt.Errorf("failed while updating status: %w: %w", statusErr, err)
		} else {
			err = fmt.Errorf("failed to update status: %w", statusErr)
		}
	}

	return err
}

func (r *Reconciler) SupportedOutput() logpipelineutils.Mode {
	return logpipelineutils.OTel
}

func (r *Reconciler) doReconcile(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline) error {
	if err := r.pipelineLock.TryAcquireLock(ctx, pipeline); err != nil {
		if errors.Is(err, resourcelock.ErrMaxPipelinesExceeded) {
			logf.FromContext(ctx).V(1).Info("Skipping reconciliation: maximum pipeline count limit exceeded")
			return nil
		}

		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	allPipelines, err := logpipelineutils.GetPipelinesForType(ctx, r.Client, r.SupportedOutput())
	if err != nil {
		return err
	}

	reconcilablePipelines, err := r.getReconcilablePipelines(ctx, allPipelines)
	if err != nil {
		return fmt.Errorf("failed to fetch deployable log pipelines: %w", err)
	}

	r.trackPipelineInfoMetric(ctx, reconcilablePipelines)

	var reconcilablePipelinesRequiringAgents = r.getPipelinesRequiringAgents(reconcilablePipelines)

	if len(reconcilablePipelinesRequiringAgents) == 0 {
		logf.FromContext(ctx).V(1).Info("cleaning up log agent resources: no log pipelines require an agent")

		if err = r.agentApplierDeleter.DeleteResources(ctx, r.Client); err != nil {
			return fmt.Errorf("failed to delete agent resources: %w", err)
		}
	}

	if len(reconcilablePipelines) == 0 {
		logf.FromContext(ctx).V(1).Info("cleaning up log pipeline resources: all log pipelines are non-reconcilable")

		if err = r.gatewayApplierDeleter.DeleteResources(ctx, r.Client, r.istioStatusChecker.IsIstioActive(ctx)); err != nil {
			return fmt.Errorf("failed to delete gateway resources: %w", err)
		}

		return nil
	}

	if err := r.reconcileLogGateway(ctx, pipeline, reconcilablePipelines); err != nil {
		return fmt.Errorf("failed to reconcile log gateway: %w", err)
	}

	if len(reconcilablePipelinesRequiringAgents) > 0 {
		if err := r.reconcileLogAgent(ctx, pipeline, reconcilablePipelinesRequiringAgents); err != nil {
			return fmt.Errorf("failed to reconcile log agent: %w", err)
		}
	}

	return nil
}

// getReconcilablePipelines returns the list of log pipelines that are ready to be rendered into the otel collector configuration.
// A pipeline is deployable if it is not being deleted, all secret references exist, and is not above the pipeline limit.
func (r *Reconciler) getReconcilablePipelines(ctx context.Context, allPipelines []telemetryv1beta1.LogPipeline) ([]telemetryv1beta1.LogPipeline, error) {
	var reconcilablePipelines []telemetryv1beta1.LogPipeline

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

func (r *Reconciler) isReconcilable(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline) (bool, error) {
	if !pipeline.GetDeletionTimestamp().IsZero() {
		return false, nil
	}

	err := r.pipelineValidator.Validate(ctx, pipeline)

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

func (r *Reconciler) reconcileLogGateway(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline, allPipelines []telemetryv1beta1.LogPipeline) error {
	shootInfo := k8sutils.GetGardenerShootInfo(ctx, r.Client)
	clusterName := r.getClusterNameFromTelemetry(ctx, shootInfo.ClusterName)

	clusterUID, err := r.getK8sClusterUID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get kube-system namespace for cluster UID: %w", err)
	}

	var enrichments *operatorv1beta1.EnrichmentSpec

	t, err := telemetryutils.GetDefaultTelemetryInstance(ctx, r.Client, r.globals.DefaultTelemetryNamespace())
	if err == nil {
		enrichments = t.Spec.Enrichments
	}

	collectorConfig, collectorEnvVars, err := r.gatewayConfigBuilder.Build(ctx, allPipelines, loggateway.BuildOptions{
		Cluster: common.ClusterOptions{
			ClusterName:   clusterName,
			ClusterUID:    clusterUID,
			CloudProvider: shootInfo.CloudProvider,
		},
		Enrichments:   enrichments,
		ModuleVersion: r.globals.Version(),
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

func (r *Reconciler) reconcileLogAgent(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline, allPipelines []telemetryv1beta1.LogPipeline) error {
	var enrichments *operatorv1beta1.EnrichmentSpec

	t, err := telemetryutils.GetDefaultTelemetryInstance(ctx, r.Client, r.globals.DefaultTelemetryNamespace())
	if err == nil {
		enrichments = t.Spec.Enrichments
	}

	shootInfo := k8sutils.GetGardenerShootInfo(ctx, r.Client)
	clusterName := r.getClusterNameFromTelemetry(ctx, shootInfo.ClusterName)

	clusterUID, err := r.getK8sClusterUID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get kube-system namespace for cluster UID: %w", err)
	}

	agentConfig, envVars, err := r.agentConfigBuilder.Build(ctx, allPipelines, logagent.BuildOptions{
		InstrumentationScopeVersion: r.globals.Version(),
		AgentNamespace:              r.globals.TargetNamespace(),
		Cluster: common.ClusterOptions{
			ClusterName:   clusterName,
			ClusterUID:    clusterUID,
			CloudProvider: shootInfo.CloudProvider,
		},
		Enrichments: enrichments,
	})
	if err != nil {
		return fmt.Errorf("failed to build agent config: %w", err)
	}

	agentConfigYAML, err := yaml.Marshal(agentConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal agent config: %w", err)
	}

	isIstioActive := r.istioStatusChecker.IsIstioActive(ctx)

	if err := r.agentApplierDeleter.ApplyResources(
		ctx,
		k8sutils.NewOwnerReferenceSetter(r.Client, pipeline),
		otelcollector.AgentApplyOptions{
			IstioEnabled:        isIstioActive,
			CollectorConfigYAML: string(agentConfigYAML),
			CollectorEnvVars:    envVars,
		},
	); err != nil {
		return fmt.Errorf("failed to apply agent resources: %w", err)
	}

	return nil
}

func (r *Reconciler) getReplicaCountFromTelemetry(ctx context.Context) int32 {
	telemetry, err := telemetryutils.GetDefaultTelemetryInstance(ctx, r.Client, r.globals.DefaultTelemetryNamespace())
	if err != nil {
		logf.FromContext(ctx).V(1).Error(err, "Failed to get telemetry: using default scaling")
		return defaultReplicaCount
	}

	if telemetry.Spec.Log != nil &&
		telemetry.Spec.Log.Gateway.Scaling.Type == operatorv1beta1.StaticScalingStrategyType &&
		telemetry.Spec.Log.Gateway.Scaling.Static != nil && telemetry.Spec.Log.Gateway.Scaling.Static.Replicas > 0 {
		return telemetry.Spec.Log.Gateway.Scaling.Static.Replicas
	}

	return defaultReplicaCount
}

func (r *Reconciler) getClusterNameFromTelemetry(ctx context.Context, defaultName string) string {
	telemetry, err := telemetryutils.GetDefaultTelemetryInstance(ctx, r.Client, r.globals.DefaultTelemetryNamespace())
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

func (r *Reconciler) getPipelinesRequiringAgents(allPipelines []telemetryv1beta1.LogPipeline) []telemetryv1beta1.LogPipeline {
	var pipelinesRequiringAgents = make([]telemetryv1beta1.LogPipeline, 0)

	for i := range allPipelines {
		if isLogAgentRequired(&allPipelines[i]) {
			pipelinesRequiringAgents = append(pipelinesRequiringAgents, allPipelines[i])
		}
	}

	return pipelinesRequiringAgents
}

func isLogAgentRequired(pipeline *telemetryv1beta1.LogPipeline) bool {
	input := pipeline.Spec.Input

	return input.Runtime != nil && input.Runtime.Enabled != nil && *input.Runtime.Enabled
}

func (r *Reconciler) trackPipelineInfoMetric(ctx context.Context, pipelines []telemetryv1beta1.LogPipeline) {
	for i := range pipelines {
		pipeline := &pipelines[i]

		var features []string

		// General features
		if sharedtypesutils.IsOTLPInputEnabled(pipeline.Spec.Input.OTLP) {
			features = append(features, metrics.FeatureInputOTLP)
		}

		if sharedtypesutils.IsTransformDefined(pipeline.Spec.Transforms) {
			features = append(features, metrics.FeatureTransform)
		}

		if sharedtypesutils.IsFilterDefined(pipeline.Spec.Filters) {
			features = append(features, metrics.FeatureFilter)
		}

		if logpipelineutils.IsRuntimeInputEnabled(&pipeline.Spec.Input) {
			features = append(features, metrics.FeatureInputRuntime)
		}

		// Get endpoint
		endpoint := r.getEndpoint(ctx, pipeline)

		// Record info metric
		metrics.RecordLogPipelineInfo(pipeline.Name, endpoint, features...)
	}
}

func (r *Reconciler) getEndpoint(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline) string {
	if pipeline.Spec.Output.OTLP == nil {
		return ""
	}

	endpointBytes, err := sharedtypesutils.ResolveValue(ctx, r.Client, pipeline.Spec.Output.OTLP.Endpoint)
	if err != nil {
		return ""
	}

	return string(endpointBytes)
}
