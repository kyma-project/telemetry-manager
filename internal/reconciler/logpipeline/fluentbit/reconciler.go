package fluentbit

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	fbports "github.com/kyma-project/telemetry-manager/internal/fluentbit/ports"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
	telemetryutils "github.com/kyma-project/telemetry-manager/internal/utils/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

type AgentConfigBuilder interface {
	Build(ctx context.Context, reconcilablePipelines []telemetryv1alpha1.LogPipeline, clusterName string) (*builder.FluentBitConfig, error)
}

type AgentApplierDeleter interface {
	ApplyResources(ctx context.Context, c client.Client, opts fluentbit.AgentApplyOptions) error
	DeleteResources(ctx context.Context, c client.Client) error
}

type IstioStatusChecker interface {
	IsIstioActive(ctx context.Context) bool
}

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

type PipelineValidator interface {
	Validate(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error
}

type ErrorToMessageConverter interface {
	Convert(err error) string
}

type FlowHealthProber interface {
	Probe(ctx context.Context, pipelineName string) (prober.FluentBitProbeResult, error)
}

type AgentProber interface {
	IsReady(ctx context.Context, name types.NamespacedName) error
}

func New(globals config.Global, client client.Client, agentConfigBuilder AgentConfigBuilder, agentApplierDeleter AgentApplierDeleter, agentProber AgentProber, healthProber FlowHealthProber, checker IstioStatusChecker, pipelineLock PipelineLock, validator PipelineValidator, converter ErrorToMessageConverter) *Reconciler {
	return &Reconciler{
		globals:             globals,
		Client:              client,
		agentConfigBuilder:  agentConfigBuilder,
		agentApplierDeleter: agentApplierDeleter,
		agentProber:         agentProber,
		flowHealthProber:    healthProber,
		istioStatusChecker:  checker,
		pipelineLock:        pipelineLock,
		pipelineValidator:   validator,
		errToMsgConverter:   converter,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
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

func (r *Reconciler) IsReconcilable(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) (bool, error) {
	if !pipeline.GetDeletionTimestamp().IsZero() {
		return false, nil
	}

	var appInputEnabled *bool

	// Treat the pipeline as non-reconcilable if the application input is explicitly disabled
	if pipeline.Spec.Input.Application != nil {
		appInputEnabled = pipeline.Spec.Input.Application.Enabled
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

func (r *Reconciler) doReconcile(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
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
func (r *Reconciler) getReconcilablePipelines(ctx context.Context, allPipelines []telemetryv1alpha1.LogPipeline) ([]telemetryv1alpha1.LogPipeline, error) {
	var reconcilableLogPipelines []telemetryv1alpha1.LogPipeline

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
