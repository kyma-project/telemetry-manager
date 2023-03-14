package logpipeline

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
)

const (
	reasonFluentBitDSNotReady     = "FluentBitDaemonSetNotReady"
	reasonFluentBitDSReady        = "FluentBitDaemonSetReady"
	reasonReferencedSecretMissing = "ReferencedSecretMissing"
)

func (r *Reconciler) updateStatus(ctx context.Context, pipelineName string) error {
	if err := r.updateStatusUnsupportedMode(ctx, pipelineName); err != nil {
		return err
	}
	return r.updateStatusConditions(ctx, pipelineName)

}

func (r *Reconciler) updateStatusUnsupportedMode(ctx context.Context, pipelineName string) error {
	var pipeline telemetryv1alpha1.LogPipeline
	if err := r.Get(ctx, types.NamespacedName{Name: pipelineName}, &pipeline); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("failed to get LogPipeline: %v", err)
	}

	desiredUnsupportedMode := pipeline.ContainsCustomPlugin()
	if pipeline.Status.UnsupportedMode != desiredUnsupportedMode {
		pipeline.Status.UnsupportedMode = desiredUnsupportedMode
		if err := r.Status().Update(ctx, &pipeline); err != nil {
			return fmt.Errorf("failed to update LogPipeline unsupported mode status: %v", err)
		}
	}

	return nil
}

func (r *Reconciler) updateStatusConditions(ctx context.Context, pipelineName string) error {
	var pipeline telemetryv1alpha1.LogPipeline
	if err := r.Get(ctx, types.NamespacedName{Name: pipelineName}, &pipeline); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("failed to get LogPipeline: %v", err)
	}

	if pipeline.DeletionTimestamp != nil {
		return nil
	}

	log := logf.FromContext(ctx)
	referencesNonExistentSecret := secretref.ReferencesNonExistentSecret(ctx, r.Client, &pipeline)
	if referencesNonExistentSecret {
		pending := telemetryv1alpha1.NewLogPipelineCondition(reasonReferencedSecretMissing, telemetryv1alpha1.LogPipelinePending)

		if pipeline.Status.HasCondition(telemetryv1alpha1.LogPipelineRunning) {
			log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s. Resetting previous conditions", pipeline.Name, pending.Type))
			pipeline.Status.Conditions = []telemetryv1alpha1.LogPipelineCondition{}
		}

		return setCondition(ctx, r.Client, &pipeline, pending)
	}

	fluentBitReady, err := r.prober.IsReady(ctx, r.config.DaemonSet)
	if err != nil {
		return err
	}

	if fluentBitReady {
		if pipeline.Status.HasCondition(telemetryv1alpha1.LogPipelineRunning) {
			return nil
		}

		running := telemetryv1alpha1.NewLogPipelineCondition(reasonFluentBitDSReady, telemetryv1alpha1.LogPipelineRunning)
		return setCondition(ctx, r.Client, &pipeline, running)
	}

	pending := telemetryv1alpha1.NewLogPipelineCondition(reasonFluentBitDSNotReady, telemetryv1alpha1.LogPipelinePending)

	if pipeline.Status.HasCondition(telemetryv1alpha1.LogPipelineRunning) {
		log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s. Resetting previous conditions", pipeline.Name, pending.Type))
		pipeline.Status.Conditions = []telemetryv1alpha1.LogPipelineCondition{}
	}

	return setCondition(ctx, r.Client, &pipeline, pending)
}

func setCondition(ctx context.Context, client client.Client, pipeline *telemetryv1alpha1.LogPipeline, condition *telemetryv1alpha1.LogPipelineCondition) error {
	log := logf.FromContext(ctx)

	log.V(1).Info(fmt.Sprintf("Updating the status of %s to %s", pipeline.Name, condition.Type))

	pipeline.Status.SetCondition(*condition)

	if err := client.Status().Update(ctx, pipeline); err != nil {
		return fmt.Errorf("failed to update LogPipeline status to %s: %v", condition.Type, err)
	}
	return nil
}
