package metricpipeline

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/collector"
)

const (
	reasonMetricGatewayDeploymentNotReady = "MetricGatewayDeploymentNotReady"
	reasonMetricGatewayDeploymentReady    = "MetricGatewayDeploymentReady"
	reasonReferencedSecretMissingReason   = "ReferencedSecretMissing"
	reasonWaitingForLock                  = "WaitingForLock"
)

func (r *Reconciler) updateStatus(ctx context.Context, pipelineName string, lockAcquired bool) error {
	log := logf.FromContext(ctx)

	var pipeline telemetryv1alpha1.MetricPipeline
	if err := r.Get(ctx, types.NamespacedName{Name: pipelineName}, &pipeline); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("failed to get MetricPipeline: %v", err)
	}

	if pipeline.DeletionTimestamp != nil {
		return nil
	}

	if !lockAcquired {
		pending := telemetryv1alpha1.NewMetricPipelineCondition(reasonWaitingForLock, telemetryv1alpha1.MetricPipelinePending)

		if pipeline.Status.HasCondition(telemetryv1alpha1.MetricPipelineRunning) {
			log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s. Resetting previous conditions", pipeline.Name, pending.Type))
			pipeline.Status.Conditions = []telemetryv1alpha1.MetricPipelineCondition{}
		}

		return setCondition(ctx, r.Client, &pipeline, pending)
	}

	secretsMissing := collector.CheckForMissingSecrets(ctx, r.Client, pipeline.Name, pipeline.Spec.Output.Otlp)
	if secretsMissing {
		pending := telemetryv1alpha1.NewMetricPipelineCondition(reasonReferencedSecretMissingReason, telemetryv1alpha1.MetricPipelinePending)

		if pipeline.Status.HasCondition(telemetryv1alpha1.MetricPipelineRunning) {
			log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s. Resetting previous conditions", pipeline.Name, pending.Type))
			pipeline.Status.Conditions = []telemetryv1alpha1.MetricPipelineCondition{}
		}

		return setCondition(ctx, r.Client, &pipeline, pending)
	}

	openTelemetryReady, err := r.prober.IsReady(ctx, types.NamespacedName{Name: r.config.BaseName, Namespace: r.config.Namespace})
	if err != nil {
		return err
	}

	if openTelemetryReady {
		if pipeline.Status.HasCondition(telemetryv1alpha1.MetricPipelineRunning) {
			return nil
		}

		running := telemetryv1alpha1.NewMetricPipelineCondition(reasonMetricGatewayDeploymentReady, telemetryv1alpha1.MetricPipelineRunning)
		return setCondition(ctx, r.Client, &pipeline, running)
	}

	pending := telemetryv1alpha1.NewMetricPipelineCondition(reasonMetricGatewayDeploymentNotReady, telemetryv1alpha1.MetricPipelinePending)

	if pipeline.Status.HasCondition(telemetryv1alpha1.MetricPipelineRunning) {
		log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s. Resetting previous conditions", pipeline.Name, pending.Type))
		pipeline.Status.Conditions = []telemetryv1alpha1.MetricPipelineCondition{}
	}

	return setCondition(ctx, r.Client, &pipeline, pending)
}

func setCondition(ctx context.Context, client client.Client, pipeline *telemetryv1alpha1.MetricPipeline, condition *telemetryv1alpha1.MetricPipelineCondition) error {
	log := logf.FromContext(ctx)

	log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s", pipeline.Name, condition.Type))

	pipeline.Status.SetCondition(*condition)

	if err := client.Status().Update(ctx, pipeline); err != nil {
		return fmt.Errorf("failed to update MetricPipeline status to %s: %v", condition.Type, err)
	}
	return nil
}
