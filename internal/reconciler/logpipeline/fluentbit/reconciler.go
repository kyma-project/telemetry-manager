package fluentbit

import (
	"context"
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
	fbports "github.com/kyma-project/telemetry-manager/internal/fluentbit/ports"
	"github.com/kyma-project/telemetry-manager/internal/metrics"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
	telemetryutils "github.com/kyma-project/telemetry-manager/internal/utils/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

// var _ logpipeline.LogPipelineReconciler = &Reconciler{}

type Reconciler struct {
	client.Client

	globals config.Global

	// Dependencies
	agentConfigBuilder  AgentConfigBuilder
	agentApplierDeleter AgentApplierDeleter
	agentProber         AgentProber
	flowHealthProber    FlowHealthProber
	istioStatusChecker  IstioStatusChecker
	pipelineLock        PipelineLock
	pipelineValidator   PipelineValidator
	errToMsgConverter   ErrorToMessageConverter
}

func (r *Reconciler) SupportedOutput() logpipelineutils.Mode {
	return logpipelineutils.FluentBit
}

// Option is a functional option for configuring a Reconciler.
type Option func(*Reconciler)

// WithPipelineLock sets the pipeline lock.
func WithPipelineLock(lock PipelineLock) Option {
	return func(r *Reconciler) {
		r.pipelineLock = lock
	}
}

// WithGlobals sets the global configuration.
func WithGlobals(globals config.Global) Option {
	return func(r *Reconciler) {
		r.globals = globals
	}
}

// WithAgentConfigBuilder sets the agent config builder.
func WithAgentConfigBuilder(builder AgentConfigBuilder) Option {
	return func(r *Reconciler) {
		r.agentConfigBuilder = builder
	}
}

// WithAgentApplierDeleter sets the agent applier/deleter.
func WithAgentApplierDeleter(applierDeleter AgentApplierDeleter) Option {
	return func(r *Reconciler) {
		r.agentApplierDeleter = applierDeleter
	}
}

// WithAgentProber sets the agent prober.
func WithAgentProber(prober AgentProber) Option {
	return func(r *Reconciler) {
		r.agentProber = prober
	}
}

// WithFlowHealthProber sets the flow health prober.
func WithFlowHealthProber(prober FlowHealthProber) Option {
	return func(r *Reconciler) {
		r.flowHealthProber = prober
	}
}

// WithIstioStatusChecker sets the Istio status checker.
func WithIstioStatusChecker(checker IstioStatusChecker) Option {
	return func(r *Reconciler) {
		r.istioStatusChecker = checker
	}
}

// WithPipelineValidator sets the pipeline validator.
func WithPipelineValidator(validator PipelineValidator) Option {
	return func(r *Reconciler) {
		r.pipelineValidator = validator
	}
}

// WithErrorToMessageConverter sets the error to message converter.
func WithErrorToMessageConverter(converter ErrorToMessageConverter) Option {
	return func(r *Reconciler) {
		r.errToMsgConverter = converter
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

func (r *Reconciler) IsReconcilable(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline) (bool, error) {
	if r.globals.OperateInFIPSMode() {
		logf.FromContext(ctx).V(1).Info("Pipeline is not reconcilable: Fluent Bit is not supported in FIPS mode")
		return false, nil
	}

	if !pipeline.GetDeletionTimestamp().IsZero() {
		return false, nil
	}

	var appInputEnabled *bool

	// Treat the pipeline as non-reconcilable if the Runtime input is explicitly disabled
	if pipeline.Spec.Input.Runtime != nil {
		appInputEnabled = pipeline.Spec.Input.Runtime.Enabled
	}

	if appInputEnabled != nil && !*appInputEnabled {
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

	err = ensureFinalizers(ctx, r.Client, pipeline)
	if err != nil {
		return err
	}

	reconcilablePipelines, err := r.getReconcilablePipelines(ctx, allPipelines)
	if err != nil {
		return fmt.Errorf("failed to fetch reconcilable log pipelines: %w", err)
	}

	if len(reconcilablePipelines) == 0 {
		logf.FromContext(ctx).V(1).Info("cleaning up log pipeline resources: all log pipelines are non-reconcilable")

		if err = r.agentApplierDeleter.DeleteResources(ctx, r.Client); err != nil {
			return fmt.Errorf("failed to delete log pipeline resources: %w", err)
		}

		if err = cleanupFinalizersIfNeeded(ctx, r.Client, pipeline); err != nil {
			return err
		}

		return nil
	}

	r.trackFeaturesUsage(reconcilablePipelines)

	shootInfo := k8sutils.GetGardenerShootInfo(ctx, r.Client)
	clusterName := r.getClusterNameFromTelemetry(ctx, shootInfo.ClusterName)

	config, err := r.agentConfigBuilder.Build(ctx, reconcilablePipelines, clusterName)
	if err != nil {
		return fmt.Errorf("failed to build fluentbit config: %w", err)
	}

	allowedPorts := getFluentBitPorts()
	if r.istioStatusChecker.IsIstioActive(ctx) {
		allowedPorts = append(allowedPorts, fbports.IstioEnvoy)
	}

	if err = r.agentApplierDeleter.ApplyResources(
		ctx,
		k8sutils.NewOwnerReferenceSetter(r.Client, pipeline),
		fluentbit.AgentApplyOptions{
			FluentBitConfig: config,
			AllowedPorts:    allowedPorts,
		},
	); err != nil {
		return err
	}

	if err = cleanupFinalizersIfNeeded(ctx, r.Client, pipeline); err != nil {
		return err
	}

	return nil
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

// getReconcilablePipelines returns the list of log pipelines that are ready to be rendered into the Fluent Bit configuration.
// A pipeline is deployable if it is not being deleted, and all secret references exist.
func (r *Reconciler) getReconcilablePipelines(ctx context.Context, allPipelines []telemetryv1beta1.LogPipeline) ([]telemetryv1beta1.LogPipeline, error) {
	var reconcilableLogPipelines []telemetryv1beta1.LogPipeline

	for i := range allPipelines {
		isReconcilable, err := r.IsReconcilable(ctx, &allPipelines[i])
		if err != nil {
			return nil, err
		}

		if isReconcilable {
			reconcilableLogPipelines = append(reconcilableLogPipelines, allPipelines[i])
		}
	}

	return reconcilableLogPipelines, nil
}

func getFluentBitPorts() []int32 {
	return []int32{
		fbports.ExporterMetrics,
		fbports.HTTP,
	}
}

func (r *Reconciler) trackFeaturesUsage(pipelines []telemetryv1beta1.LogPipeline) {
	for i := range pipelines {
		// General features
		if logpipelineutils.IsRuntimeInputEnabled(&pipelines[i].Spec.Input) {
			metrics.RecordLogPipelineFeatureUsage(metrics.FeatureInputRuntime, pipelines[i].Name)
		}

		// FluentBit features

		if logpipelineutils.IsCustomFilterDefined(pipelines[i].Spec.FluentBitFilters) {
			metrics.RecordLogPipelineFeatureUsage(metrics.FeatureFilters, pipelines[i].Name)
		}

		if logpipelineutils.IsCustomOutputDefined(&pipelines[i].Spec.Output) {
			metrics.RecordLogPipelineFeatureUsage(metrics.FeatureOutputCustom, pipelines[i].Name)
		}

		if logpipelineutils.IsHTTPOutputDefined(&pipelines[i].Spec.Output) {
			metrics.RecordLogPipelineFeatureUsage(metrics.FeatureOutputHTTP, pipelines[i].Name)
		}

		if logpipelineutils.IsVariablesDefined(pipelines[i].Spec.FluentBitVariables) {
			metrics.RecordLogPipelineFeatureUsage(metrics.FeatureVariables, pipelines[i].Name)
		}

		if logpipelineutils.IsFilesDefined(pipelines[i].Spec.FluentBitFiles) {
			metrics.RecordLogPipelineFeatureUsage(metrics.FeatureFiles, pipelines[i].Name)
		}
	}
}
