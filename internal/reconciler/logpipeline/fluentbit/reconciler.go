package fluentbit

import (
	"context"
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/ports"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline"
	"github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

const (
	defaultInputTag          = "tele"
	defaultMemoryBufferLimit = "10M"
	defaultStorageType       = "filesystem"
	defaultFsBufferLimit     = "1G"
)

type AgentApplierDeleter interface {
	ApplyResources(ctx context.Context, c client.Client, opts fluentbit.AgentApplyOptions) error
	DeleteResources(ctx context.Context, c client.Client, opts fluentbit.AgentApplyOptions) error
}

type IstioStatusChecker interface {
	IsIstioActive(ctx context.Context) bool
}

var _ logpipeline.LogPipelineReconciler = &Reconciler{}

type Reconciler struct {
	client.Client

	config fluentbit.Config

	agentApplierDeleter AgentApplierDeleter

	// Dependencies
	agentProber        commonstatus.Prober
	flowHealthProber   logpipeline.FlowHealthProber
	istioStatusChecker IstioStatusChecker
	pipelineValidator  *Validator
	errToMsgConverter  commonstatus.ErrorToMessageConverter
}

func (r *Reconciler) SupportedOutput() logpipelineutils.Mode {
	return logpipelineutils.FluentBit
}

func New(client client.Client, config fluentbit.Config, agentApplierDeleter AgentApplierDeleter, prober commonstatus.Prober, healthProber logpipeline.FlowHealthProber, checker IstioStatusChecker, validator *Validator, converter commonstatus.ErrorToMessageConverter) *Reconciler {
	config.PipelineDefaults = builder.PipelineDefaults{
		InputTag:          defaultInputTag,
		MemoryBufferLimit: defaultMemoryBufferLimit,
		StorageType:       defaultStorageType,
		FsBufferLimit:     defaultFsBufferLimit,
	}

	return &Reconciler{
		Client:              client,
		agentApplierDeleter: agentApplierDeleter,
		config:              config,
		agentProber:         prober,
		flowHealthProber:    healthProber,
		istioStatusChecker:  checker,
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

func (r *Reconciler) doReconcile(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
	allPipelines, err := logpipeline.GetPipelinesForType(ctx, r.Client, r.SupportedOutput())
	if err != nil {
		return err
	}

	err = ensureFinalizers(ctx, r.Client, pipeline)
	if err != nil {
		return err
	}

	reconcilablePipelines, err := r.getReconcilablePipelines(ctx, allPipelines)
	logf.FromContext(ctx).V(1).Info("reconcilable pipelines: %s", len(reconcilablePipelines))
	if err != nil {
		return fmt.Errorf("failed to fetch reconcilable log pipelines: %w", err)
	}

	if len(reconcilablePipelines) == 0 {
		logf.FromContext(ctx).V(1).Info("cleaning up log pipeline resources: all log pipelines are non-reconcilable")

		if err = r.agentApplierDeleter.DeleteResources(ctx, r.Client, fluentbit.AgentApplyOptions{Config: r.config}); err != nil {
			return fmt.Errorf("failed to delete log pipeline resources: %w", err)
		}

		if err = cleanupFinalizersIfNeeded(ctx, r.Client, pipeline); err != nil {
			return err
		}

		return nil
	}

	allowedPorts := getFluentBitPorts()
	if r.istioStatusChecker.IsIstioActive(ctx) {
		allowedPorts = append(allowedPorts, ports.IstioEnvoy)
	}

	if err = r.agentApplierDeleter.ApplyResources(
		ctx,
		k8sutils.NewOwnerReferenceSetter(r.Client, pipeline),
		fluentbit.AgentApplyOptions{
			Config:                 r.config,
			AllowedPorts:           allowedPorts,
			Pipeline:               pipeline,
			DeployableLogPipelines: reconcilablePipelines,
		},
	); err != nil {
		return err
	}

	return nil
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

func getFluentBitPorts() []int32 {
	return []int32{
		ports.ExporterMetrics,
		ports.HTTP,
	}
}
