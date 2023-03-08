package telemetry

import (
	"context"
	"fmt"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/kubernetes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	reasonTraceCollectorDeploymentNotReady = "TraceCollectorDeploymentNotReady"
	reasonTraceCollectorDeploymentReady    = "TraceCollectorDeploymentReady"
	reasonReferencedSecretMissingReason    = "ReferencedSecretMissing"
	reasonWaitingForLock                   = "WaitingForLock"
)

func (r *MetricPipelineReconciler) updateStatus(ctx context.Context, pipelineName string, lockAcquired bool) error {
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

	secretsMissing := checkForMissingSecrets(ctx, r.Client, &pipeline)
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

		running := telemetryv1alpha1.NewMetricPipelineCondition(reasonTraceCollectorDeploymentReady, telemetryv1alpha1.MetricPipelineRunning)
		return setCondition(ctx, r.Client, &pipeline, running)
	}

	pending := telemetryv1alpha1.NewMetricPipelineCondition(reasonTraceCollectorDeploymentNotReady, telemetryv1alpha1.MetricPipelinePending)

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

func checkForMissingSecrets(ctx context.Context, client client.Client, pipeline *telemetryv1alpha1.MetricPipeline) bool {
	secretRefFields := kubernetes.LookupSecretRefFields(pipeline.Spec.Output.Otlp, pipeline.Name)
	for _, field := range secretRefFields {
		hasKey := checkSecretHasKey(ctx, client, field.SecretKeyRef)
		if !hasKey {
			return true
		}
	}

	return false
}

func checkSecretHasKey(ctx context.Context, client client.Client, from telemetryv1alpha1.SecretKeyRef) bool {
	log := logf.FromContext(ctx)

	var secret corev1.Secret
	if err := client.Get(ctx, types.NamespacedName{Name: from.Name, Namespace: from.Namespace}, &secret); err != nil {
		log.V(1).Info(fmt.Sprintf("Unable to get secret '%s' from namespace '%s'", from.Name, from.Namespace))
		return false
	}
	if _, ok := secret.Data[from.Key]; !ok {
		log.V(1).Info(fmt.Sprintf("Unable to find key '%s' in secret '%s'", from.Key, from.Name))
		return false
	}

	return true
}
