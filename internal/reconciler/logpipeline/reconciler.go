package logpipeline

import (
	"context"
	"errors"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
)

var (
	ErrUnsupportedOutputType = fmt.Errorf("unsupported output type")
)

type Reconciler struct {
	client.Client

	globals          config.Global
	overridesHandler OverridesHandler
	reconcilers      map[logpipelineutils.Mode]LogPipelineReconciler

	pipelineSyncer PipelineSyncer
	secretWatcher  SecretWatcher
}

// Option is a functional option for configuring a Reconciler.
type Option func(*Reconciler)

// WithGlobals sets the global configuration.
func WithGlobals(globals config.Global) Option {
	return func(r *Reconciler) {
		r.globals = globals
	}
}

// WithOverridesHandler sets the overrides handler.
func WithOverridesHandler(handler OverridesHandler) Option {
	return func(r *Reconciler) {
		r.overridesHandler = handler
	}
}

// WithPipelineSyncer sets the pipeline syncer.
func WithPipelineSyncer(syncer PipelineSyncer) Option {
	return func(r *Reconciler) {
		r.pipelineSyncer = syncer
	}
}

// WithReconcilers sets the pipeline reconcilers.
func WithReconcilers(reconcilers ...LogPipelineReconciler) Option {
	return func(r *Reconciler) {
		reconcilersMap := make(map[logpipelineutils.Mode]LogPipelineReconciler)
		for _, rec := range reconcilers {
			reconcilersMap[rec.SupportedOutput()] = rec
		}

		r.reconcilers = reconcilersMap
	}
}

// WithSecretWatcher sets the secret watcher.
func WithSecretWatcher(watcher SecretWatcher) Option {
	return func(r *Reconciler) {
		r.secretWatcher = watcher
	}
}

// New creates a new Reconciler with the provided client and functional options.
// All dependencies must be provided via functional options.
func New(client client.Client, opts ...Option) *Reconciler {
	r := &Reconciler{
		Client:      client,
		reconcilers: make(map[logpipelineutils.Mode]LogPipelineReconciler),
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logf.FromContext(ctx).V(1).Info("Reconciling LogPipeline")

	overrideConfig, err := r.overridesHandler.LoadOverrides(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	if overrideConfig.Logging.Paused {
		logf.FromContext(ctx).V(1).Info("Skipping reconciliation: paused using override config")
		return ctrl.Result{}, nil
	}

	var pipeline telemetryv1beta1.LogPipeline
	if err := r.Get(ctx, req.NamespacedName, &pipeline); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}

		// Pipeline was deleted, clean up secret watchers
		if err := r.secretWatcher.RemoveFromWatchers(ctx, req.Name, telemetryv1beta1.GroupVersion.WithKind("LogPipeline")); err != nil {
			return ctrl.Result{}, err
		}

		// Remove pipeline reference from OTLP Gateway ConfigMap
		if err := otelcollector.RemovePipelineReference(ctx, r.Client, r.globals.TargetNamespace(), common.SignalTypeLog, req.Name); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to remove pipeline reference from ConfigMap: %w", err)
		}

		return ctrl.Result{}, nil
	}

	if err := r.syncSecretWatchers(ctx, &pipeline); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.pipelineSyncer.TryAcquireLock(ctx, &pipeline); err != nil {
		if errors.Is(err, resourcelock.ErrMaxPipelinesExceeded) {
			logf.FromContext(ctx).V(1).Error(err, "Skipping reconciliation: max pipelines exceeded")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	outputType := logpipelineutils.GetOutputType(&pipeline)
	reconciler, ok := r.reconcilers[outputType]

	if !ok {
		return ctrl.Result{}, fmt.Errorf("%w: %v", ErrUnsupportedOutputType, outputType)
	}

	result, err := reconciler.Reconcile(ctx, &pipeline)

	return result, err
}

func (r *Reconciler) syncSecretWatchers(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline) error {
	if r.secretWatcher == nil {
		return nil
	}

	refs := secretref.GetSecretRefsLogPipeline(pipeline)
	secrets := secretref.RefsToSecretNames(refs)

	return r.secretWatcher.SyncWatchers(ctx, pipeline, secrets)
}
