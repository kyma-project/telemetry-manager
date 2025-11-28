package fluentbit

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
	fbports "github.com/kyma-project/telemetry-manager/internal/fluentbit/ports"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
	telemetryutils "github.com/kyma-project/telemetry-manager/internal/utils/telemetry"
)

// var _ logpipeline.LogPipelineReconciler = &Reconciler{}

// ReconcileResult holds the results of a reconciliation that can be reused for status updates.
type ReconcileResult struct {
	// ValidationError is the error returned from validating the pipeline, or nil if validation succeeded.
	ValidationError error
	// IsLockHolder indicates whether the pipeline successfully holds a lock after reconciliation.
	IsLockHolder bool
}

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

func (r *Reconciler) Reconcile(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
	logf.FromContext(ctx).V(1).Info("Reconciling LogPipeline")

	result, err := r.doReconcile(ctx, pipeline)
	if statusErr := r.updateStatus(ctx, pipeline.Name, result); statusErr != nil {
		if err != nil {
			err = fmt.Errorf("failed while updating status: %w: %w", statusErr, err)
		} else {
			err = fmt.Errorf("failed to update status: %w", statusErr)
		}
	}

	return err
}

// doReconcile performs the main reconciliation logic for a LogPipeline.
// The idea is that it always fully reconciles the state, regardless of whether the pipeline is valid or not.
// It returns a ReconcileResult that contains information about the reconciliation outcome,
// which can be used for status updates.
func (r *Reconciler) doReconcile(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) (ReconcileResult, error) {
	result := ReconcileResult{}

	if err := r.validateAndManageLock(ctx, pipeline, &result); err != nil {
		return result, err
	}

	if result.IsLockHolder {
		if err := ensureFinalizers(ctx, r.Client, pipeline); err != nil {
			return result, err
		}
	}

	reconcilablePipelines, err := r.getReconcilablePipelines(ctx)
	if err != nil {
		return result, fmt.Errorf("failed to fetch reconcilable log pipelines: %w", err)
	}

	if len(reconcilablePipelines) == 0 {
		return result, r.cleanupResources(ctx, pipeline)
	}

	if err := r.deployFluentBit(ctx, pipeline, reconcilablePipelines); err != nil {
		return result, err
	}

	if err := cleanupFinalizersIfNeeded(ctx, r.Client, pipeline); err != nil {
		return result, err
	}

	return result, nil
}

// validateAndManageLock validates the pipeline and manages the lock based on the validation result and FIPS mode.
// It updates the result with ValidationError and IsLockHolder status.
func (r *Reconciler) validateAndManageLock(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline, result *ReconcileResult) error {
	validationErr := r.pipelineValidator.Validate(ctx, pipeline)
	result.ValidationError = validationErr

	// FIPS mode: always release lock
	if r.globals.OperateInFIPSMode() {
		if err := r.pipelineLock.ReleaseLockIfHeld(ctx, pipeline); err != nil {
			return fmt.Errorf("failed to release lock: %w", err)
		}
		result.IsLockHolder = false
		return nil
	}

	// Validation succeeded: try to acquire lock
	if validationErr == nil {
		return r.tryAcquireLock(ctx, pipeline, result)
	}

	// Validation failed: handle the error
	return r.handleValidationFailure(ctx, validationErr, result)
}

// tryAcquireLock attempts to acquire the pipeline lock.
// If max pipelines exceeded, it logs and continues without the lock.
func (r *Reconciler) tryAcquireLock(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline, result *ReconcileResult) error {
	if err := r.pipelineLock.TryAcquireLock(ctx, pipeline); err != nil {
		if errors.Is(err, resourcelock.ErrMaxPipelinesExceeded) {
			logf.FromContext(ctx).V(1).Info("Skipping reconciliation: maximum pipeline count limit exceeded")
			result.IsLockHolder = false
			return nil
		}
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	result.IsLockHolder = true
	return nil
}

// handleValidationFailure logs the validation error and determines if we should requeue.
func (r *Reconciler) handleValidationFailure(ctx context.Context, validationErr error, result *ReconcileResult) error {
	msg := r.errToMsgConverter.Convert(validationErr)
	logf.FromContext(ctx).V(1).Info("Validation failed", "error", msg)

	result.IsLockHolder = false

	// If validation failed due to an API request error, return the error to trigger a requeue
	var APIRequestFailed *errortypes.APIRequestFailedError
	if errors.As(validationErr, &APIRequestFailed) {
		return APIRequestFailed.Err
	}

	return nil
}

// cleanupResources removes all FluentBit resources when there are no reconcilable pipelines.
func (r *Reconciler) cleanupResources(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
	logf.FromContext(ctx).V(1).Info("cleaning up log pipeline resources: all log pipelines are non-reconcilable")

	if err := r.agentApplierDeleter.DeleteResources(ctx, r.Client); err != nil {
		return fmt.Errorf("failed to delete log pipeline resources: %w", err)
	}

	if err := cleanupFinalizersIfNeeded(ctx, r.Client, pipeline); err != nil {
		return err
	}

	return nil
}

// deployFluentBit builds the FluentBit configuration and deploys the agent resources.
func (r *Reconciler) deployFluentBit(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline, reconcilablePipelines []telemetryv1alpha1.LogPipeline) error {
	// Build FluentBit configuration
	shootInfo := k8sutils.GetGardenerShootInfo(ctx, r.Client)
	clusterName := r.getClusterNameFromTelemetry(ctx, shootInfo.ClusterName)

	fbConfig, err := r.agentConfigBuilder.Build(ctx, reconcilablePipelines, clusterName)
	if err != nil {
		return fmt.Errorf("failed to build fluentbit config: %w", err)
	}

	// Determine allowed ports
	allowedPorts := getFluentBitPorts()
	if r.istioStatusChecker.IsIstioActive(ctx) {
		allowedPorts = append(allowedPorts, fbports.IstioEnvoy)
	}

	// Apply resources
	if err := r.agentApplierDeleter.ApplyResources(
		ctx,
		k8sutils.NewOwnerReferenceSetter(r.Client, pipeline),
		fluentbit.AgentApplyOptions{
			FluentBitConfig: fbConfig,
			AllowedPorts:    allowedPorts,
		},
	); err != nil {
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
func (r *Reconciler) getReconcilablePipelines(ctx context.Context) ([]telemetryv1alpha1.LogPipeline, error) {
	// In FIPS mode, Fluent Bit is not supported, so return empty list to trigger cleanup
	if r.globals.OperateInFIPSMode() {
		return []telemetryv1alpha1.LogPipeline{}, nil
	}

	var pll telemetryv1alpha1.LogPipelineList
	if err := r.pipelineLock.GetLockHolders(ctx, &pll); err != nil {
		return nil, fmt.Errorf("failed to get reconcilable pipelines: %w", err)
	}

	allPipelines, err := meta.ExtractList(&pll)
	if err != nil {
		return nil, fmt.Errorf("failed to extract log pipelines from list: %w", err)
	}

	var reconcilableLogPipelines []telemetryv1alpha1.LogPipeline

	for _, pl := range allPipelines {
		logPipeline, ok := pl.(*telemetryv1alpha1.LogPipeline)
		if !ok {
			continue
		}
		// Skip pipelines that are being deleted
		if !logPipeline.GetDeletionTimestamp().IsZero() {
			continue
		}
		// Skip pipelines that do not match the supported output type
		if logpipelineutils.GetOutputType(logPipeline) != r.SupportedOutput() {
			continue
		}

		reconcilableLogPipelines = append(reconcilableLogPipelines, *logPipeline)
	}

	return reconcilableLogPipelines, nil
}

func getFluentBitPorts() []int32 {
	return []int32{
		fbports.ExporterMetrics,
		fbports.HTTP,
	}
}
