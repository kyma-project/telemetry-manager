package logpipeline

import (
	"context"
	"errors"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
)

var (
	ErrUnsupportedOutputType = fmt.Errorf("unsupported output type")
)

type Reconciler struct {
	client.Client

	overridesHandler OverridesHandler
	reconcilers      map[logpipelineutils.Mode]LogPipelineReconciler

	pipelineSyncer PipelineSyncer
}

// Option is a functional option for configuring a Reconciler.
type Option func(*Reconciler)

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
		return ctrl.Result{}, client.IgnoreNotFound(err)
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
