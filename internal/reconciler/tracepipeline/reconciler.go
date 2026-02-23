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

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
	"github.com/kyma-project/telemetry-manager/internal/metrics"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	sharedtypesutils "github.com/kyma-project/telemetry-manager/internal/utils/sharedtypes"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

// defaultReplicaCount is no longer used in the new architecture.
// The OTLP Gateway Controller manages deployment configuration.
const defaultReplicaCount int32 = 2

type Reconciler struct {
	client.Client

	globals config.Global

	// Dependencies
	flowHealthProber  FlowHealthProber
	overridesHandler  OverridesHandler
	pipelineLock      PipelineLock
	pipelineSync      PipelineSyncer
	pipelineValidator *Validator
	errToMsgConverter commonstatus.ErrorToMessageConverter
}

// Option configures the Reconciler during initialization.
type Option func(*Reconciler)

// WithGlobals sets the global configuration.
func WithGlobals(globals config.Global) Option {
	return func(r *Reconciler) {
		r.globals = globals
	}
}

// WithFlowHealthProber sets the flow health prober for the Reconciler.
func WithFlowHealthProber(prober FlowHealthProber) Option {
	return func(r *Reconciler) {
		r.flowHealthProber = prober
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
// It validates the pipeline and writes it to the OTLP Gateway ConfigMap if reconcilable,
// or removes it from the ConfigMap if not reconcilable.
func (r *Reconciler) doReconcile(ctx context.Context, pipeline *telemetryv1beta1.TracePipeline) error {
	if err := r.pipelineLock.TryAcquireLock(ctx, pipeline); err != nil {
		if errors.Is(err, resourcelock.ErrMaxPipelinesExceeded) {
			logf.FromContext(ctx).V(1).Info("Skipping reconciliation: maximum pipeline count limit exceeded")
			return nil
		}

		return err
	}

	// Track metrics for all pipelines
	var allPipelinesList telemetryv1beta1.TracePipelineList
	if err := r.List(ctx, &allPipelinesList); err != nil {
		return fmt.Errorf("failed to list trace pipelines: %w", err)
	}

	r.trackPipelineInfoMetric(ctx, allPipelinesList.Items)

	// Validate pipeline
	isReconcilable, err := r.isReconcilable(ctx, pipeline)
	if err != nil {
		return fmt.Errorf("failed to validate pipeline: %w", err)
	}

	// Update ConfigMap based on validation result
	if isReconcilable {
		// Write pipeline reference to OTLP Gateway ConfigMap
		logf.FromContext(ctx).V(1).Info("Writing pipeline reference to OTLP Gateway ConfigMap", "pipeline", pipeline.Name, "generation", pipeline.Generation)

		if err := otelcollector.WriteTracePipelineReference(
			ctx,
			r.Client,
			r.globals.TargetNamespace(),
			pipeline.Name,
			pipeline.Generation,
		); err != nil {
			return fmt.Errorf("failed to write pipeline reference to ConfigMap: %w", err)
		}
	} else {
		// Remove pipeline reference from ConfigMap
		logf.FromContext(ctx).V(1).Info("Removing pipeline reference from OTLP Gateway ConfigMap", "pipeline", pipeline.Name)

		if err := otelcollector.RemoveTracePipelineReference(
			ctx,
			r.Client,
			r.globals.TargetNamespace(),
			pipeline.Name,
		); err != nil {
			return fmt.Errorf("failed to remove pipeline reference from ConfigMap: %w", err)
		}
	}

	return nil
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

func (r *Reconciler) trackPipelineInfoMetric(ctx context.Context, pipelines []telemetryv1beta1.TracePipeline) {
	for i := range pipelines {
		pipeline := &pipelines[i]

		var features []string

		// General features
		if sharedtypesutils.IsTransformDefined(pipeline.Spec.Transforms) {
			features = append(features, metrics.FeatureTransform)
		}

		if sharedtypesutils.IsFilterDefined(pipeline.Spec.Filters) {
			features = append(features, metrics.FeatureFilter)
		}

		// Get endpoint
		endpoint := r.getEndpoint(ctx, pipeline)

		// Record info metric
		metrics.RecordTracePipelineInfo(pipeline.Name, endpoint, features...)
	}
}

func (r *Reconciler) getEndpoint(ctx context.Context, pipeline *telemetryv1beta1.TracePipeline) string {
	if pipeline.Spec.Output.OTLP == nil {
		return ""
	}

	endpointBytes, err := sharedtypesutils.ResolveValue(ctx, r.Client, pipeline.Spec.Output.OTLP.Endpoint)
	if err != nil {
		return ""
	}

	return string(endpointBytes)
}
