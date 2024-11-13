package otel

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
)

type Config struct {
}

var _ logpipeline.LogPipelineReconciler = &Reconciler{}

type Reconciler struct {
	client.Client

	config Config

	errToMessageConverter commonstatus.ErrorToMessageConverter
}

func New(client client.Client, errToMessageConverter commonstatus.ErrorToMessageConverter) *Reconciler {
	return &Reconciler{
		Client:                client,
		errToMessageConverter: errToMessageConverter,
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

func (r *Reconciler) SupportedOutput() logpipelineutils.Mode {
	return logpipelineutils.OTel
}

func (r *Reconciler) doReconcile(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
	// TODO: Do I need a lock, as for metric and trace pipelines? No, will be handled in a different task.

	allPipelines, err := logpipeline.GetPipelinesForType(ctx, r.Client, r.SupportedOutput())
	if err != nil {
		return err
	}

	// TODO: Check if no pipeline is reconcilable. Not a priority, do this later.

	if err := r.reconcileLogGateway(ctx, pipeline, allPipelines); err != nil {
		return fmt.Errorf("failed to reconcile log gateway: %w", err)
	}

	return nil
}

func (r *Reconciler) reconcileLogGateway(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline, allPipelines []telemetryv1alpha1.LogPipeline) error {
	return nil

	// TODO: Look into the config builder, see if it needs any changes for the logpipeline specifically
	// collectorConfig, collectorEnvVars, err := r.gatewayConfigBuilder.Build(ctx, allPipelines)
	// if err != nil {
	// 	return fmt.Errorf("failed to create collector config: %w", err)
	// }

	// collectorConfigYAML, err := yaml.Marshal(collectorConfig)
	// if err != nil {
	// 	return fmt.Errorf("failed to marshal collector config: %w", err)
	// }

	// TODO: Decide what's needed from these options
	// opts := otelcollector.GatewayApplyOptions{
	// 	AllowedPorts:                   allowedPorts,
	// 	CollectorConfigYAML:            string(collectorConfigYAML),
	// 	CollectorEnvVars:               collectorEnvVars,
	// 	ComponentSelectorLabels:        traceGatewaySelectorLabels,
	// 	IstioEnabled:                   isIstioActive,
	// 	IstioExcludePorts:              []int32{ports.Metrics},
	// 	Replicas:                       r.getReplicaCountFromTelemetry(ctx),
	// 	ResourceRequirementsMultiplier: len(allPipelines),
	// }

	// if err := r.gatewayApplierDeleter.ApplyResources(
	// 	ctx,
	// 	k8sutils.NewOwnerReferenceSetter(r.Client, pipeline),
	// 	opts,
	// ); err != nil {
	// 	return fmt.Errorf("failed to apply gateway resources: %w", err)
	// }

	// return nil
}
