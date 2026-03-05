package otel

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
	"github.com/kyma-project/telemetry-manager/internal/metrics"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/logagent"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
	sharedtypesutils "github.com/kyma-project/telemetry-manager/internal/utils/sharedtypes"
	telemetryutils "github.com/kyma-project/telemetry-manager/internal/utils/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

const defaultReplicaCount int32 = 2

type Reconciler struct {
	client.Client

	globals config.Global

	// Dependencies
	agentFlowHealthProber   AgentFlowHealthProber
	gatewayFlowHealthProber GatewayFlowHealthProber
	gatewayProber           Prober
	agentConfigBuilder      AgentConfigBuilder
	agentProber             Prober
	agentApplierDeleter     AgentApplierDeleter
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

// WithAgentFlowHealthProber sets the agent flow health prober.
func WithAgentFlowHealthProber(prober AgentFlowHealthProber) Option {
	return func(r *Reconciler) {
		r.agentFlowHealthProber = prober
	}
}

// WithGatewayFlowHealthProber sets the gateway flow health prober.
func WithGatewayFlowHealthProber(prober GatewayFlowHealthProber) Option {
	return func(r *Reconciler) {
		r.gatewayFlowHealthProber = prober
	}
}

// WithGatewayProber sets the gateway prober.
func WithGatewayProber(prober Prober) Option {
	return func(r *Reconciler) {
		r.gatewayProber = prober
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

func (r *Reconciler) Reconcile(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline) (ctrl.Result, error) {
	logf.FromContext(ctx).V(1).Info("Reconciling OTel LogPipeline")

	var allErrors error = nil

	if err := r.doReconcile(ctx, pipeline); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to reconcile: %w", err))
	}

	if err := r.updateStatus(ctx, pipeline.Name); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to update status: %w", err))
	}

	if allErrors != nil {
		return ctrl.Result{}, allErrors
	}

	requeueAfter := r.calculateRequeueAfterDuration(ctx, pipeline)
	if requeueAfter != nil {
		logf.FromContext(ctx).V(1).Info("Requeuing reconciliation due to certificate about to expire", "RequeueAfter", requeueAfter.String())
		return ctrl.Result{RequeueAfter: *requeueAfter}, nil
	}

	return ctrl.Result{}, nil
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

	r.trackPipelineInfoMetric(ctx, allPipelines)

	// Validate current pipeline
	isReconcilable, err := r.isReconcilable(ctx, pipeline)
	if err != nil {
		return fmt.Errorf("failed to validate pipeline: %w", err)
	}

	// Update ConfigMap based on validation result
	if isReconcilable {
		// Collect secret references and their current versions
		secretRefs := secretref.GetSecretRefsLogPipeline(pipeline)
		secretVersions := otelcollector.CollectSecretVersions(ctx, r.Client, secretRefs)

		// Write current pipeline reference to OTLP Gateway ConfigMap
		logf.FromContext(ctx).V(1).Info("Writing pipeline reference to OTLP Gateway ConfigMap",
			"pipeline", pipeline.Name,
			"generation", pipeline.Generation,
			"secretCount", len(secretVersions))

		if err := otelcollector.WritePipelineReference(ctx, r.Client, r.globals.TargetNamespace(), common.SignalTypeLog, otelcollector.PipelineReferenceInput{
			Name:           pipeline.Name,
			Generation:     pipeline.Generation,
			SecretVersions: secretVersions,
		},
		); err != nil {
			return fmt.Errorf("failed to write pipeline reference to ConfigMap: %w", err)
		}
	} else {
		// Remove current pipeline reference from ConfigMap
		logf.FromContext(ctx).V(1).Info("Removing pipeline reference from OTLP Gateway ConfigMap", "pipeline", pipeline.Name)

		if err := otelcollector.RemovePipelineReference(ctx, r.Client, r.globals.TargetNamespace(), common.SignalTypeLog, pipeline.Name); err != nil {
			return fmt.Errorf("failed to remove pipeline reference from ConfigMap: %w", err)
		}
	}

	// Get reconcilable pipelines (for agent deployment)
	reconcilablePipelines, err := r.getReconcilablePipelines(ctx, allPipelines)
	if err != nil {
		return fmt.Errorf("failed to fetch deployable log pipelines: %w", err)
	}

	var reconcilablePipelinesRequiringAgents = r.getPipelinesRequiringAgents(reconcilablePipelines)

	if len(reconcilablePipelinesRequiringAgents) == 0 {
		logf.FromContext(ctx).V(1).Info("cleaning up log agent resources: no log pipelines require an agent")

		if err = r.agentApplierDeleter.DeleteResources(ctx, r.Client); err != nil {
			return fmt.Errorf("failed to delete agent resources: %w", err)
		}

		return nil
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

func (r *Reconciler) reconcileLogAgent(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline, allPipelines []telemetryv1beta1.LogPipeline) error {
	var enrichments *operatorv1beta1.EnrichmentSpec

	t, err := telemetryutils.GetDefaultTelemetryInstance(ctx, r.Client, r.globals.DefaultTelemetryNamespace())
	if err == nil {
		enrichments = t.Spec.Enrichments
	}

	shootInfo := k8sutils.GetGardenerShootInfo(ctx, r.Client)
	telemetryOptions := telemetryutils.Options{
		SignalType:                common.SignalTypeLog,
		Client:                    r.Client,
		DefaultReplicas:           defaultReplicaCount,
		DefaultTelemetryNamespace: r.globals.DefaultTelemetryNamespace(),
	}
	clusterName := telemetryutils.GetClusterNameFromTelemetry(ctx, telemetryOptions)

	clusterUID, err := k8sutils.GetClusterUID(ctx, r.Client)
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
		Enrichments:       enrichments,
		ServiceEnrichment: telemetryutils.GetServiceEnrichmentFromTelemetryOrDefault(ctx, telemetryOptions),
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

func (r *Reconciler) calculateRequeueAfterDuration(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline) *time.Duration {
	err := r.pipelineValidator.Validate(ctx, pipeline)

	var errCertAboutToExpire *tlscert.CertAboutToExpireError
	if errors.As(err, &errCertAboutToExpire) {
		duration := time.Until(errCertAboutToExpire.Expiry)
		return &duration
	}

	return nil
}
