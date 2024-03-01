package logpipeline

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
)

func (r *Reconciler) updateStatus(ctx context.Context, pipelineName string) error {
	var pipeline telemetryv1alpha1.LogPipeline
	if err := r.Get(ctx, types.NamespacedName{Name: pipelineName}, &pipeline); err != nil {
		if apierrors.IsNotFound(err) {
			logf.FromContext(ctx).V(1).Info("Skipping status update for LogPipeline - not found")
			return nil
		}

		return fmt.Errorf("failed to get LogPipeline: %v", err)
	}

	if pipeline.DeletionTimestamp != nil {
		logf.FromContext(ctx).V(1).Info("Skipping status update for LogPipeline - marked for deletion")
		return nil
	}

	if err := r.updateStatusUnsupportedMode(ctx, &pipeline); err != nil {
		return err
	}

	// If the "AgentHealthy" type doesn't exist in the conditions,
	// then we need to reset the conditions list to ensure that the "Pending" and "Running" conditions are appended to the end of the conditions list
	// Check step 3 in https://github.com/kyma-project/telemetry-manager/blob/main/docs/contributor/arch/004-consolidate-pipeline-statuses.md#decision
	if meta.FindStatusCondition(pipeline.Status.Conditions, conditions.TypeAgentHealthy) == nil {
		pipeline.Status.Conditions = []metav1.Condition{}
	}

	r.setAgentHealthyCondition(ctx, &pipeline)
	r.setFluentBitConfigGeneratedCondition(ctx, &pipeline)
	r.setPendingAndRunningConditions(ctx, &pipeline)

	if err := r.Status().Update(ctx, &pipeline); err != nil {
		return fmt.Errorf("failed to update LogPipeline status: %w", err)
	}

	return nil
}

func (r *Reconciler) updateStatusUnsupportedMode(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
	desiredUnsupportedMode := pipeline.ContainsCustomPlugin()
	if pipeline.Status.UnsupportedMode != desiredUnsupportedMode {
		pipeline.Status.UnsupportedMode = desiredUnsupportedMode
		if err := r.Status().Update(ctx, pipeline); err != nil {
			return fmt.Errorf("failed to update LogPipeline unsupported mode status: %v", err)
		}
	}

	return nil
}

func (r *Reconciler) setAgentHealthyCondition(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) {
	healthy, err := r.prober.IsReady(ctx, r.config.DaemonSet)
	if err != nil {
		logf.FromContext(ctx).V(1).Error(err, "Failed to probe fluent bit daemonset - set condition as not healthy")
		healthy = false
	}

	status := metav1.ConditionFalse
	reason := conditions.ReasonDaemonSetNotReady
	if healthy {
		status = metav1.ConditionTrue
		reason = conditions.ReasonDaemonSetReady
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, conditions.New(conditions.TypeAgentHealthy, reason, status, pipeline.Generation, conditions.LogsMessage))
}

func (r *Reconciler) setFluentBitConfigGeneratedCondition(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) {
	status := metav1.ConditionTrue
	reason := conditions.ReasonConfigurationGenerated

	if secretref.ReferencesNonExistentSecret(ctx, r.Client, pipeline) {
		status = metav1.ConditionFalse
		reason = conditions.ReasonReferencedSecretMissing
	}

	if pipeline.Spec.Output.IsLokiDefined() {
		status = metav1.ConditionFalse
		reason = conditions.ReasonUnsupportedLokiOutput
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, conditions.New(conditions.TypeConfigurationGenerated, reason, status, pipeline.Generation, conditions.LogsMessage))
}

func (r *Reconciler) setPendingAndRunningConditions(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) {

	if pipeline.Spec.Output.IsLokiDefined() {
		conditions.SetPendingCondition(ctx, &pipeline.Status.Conditions, pipeline.Generation, conditions.ReasonUnsupportedLokiOutput, pipeline.Name, conditions.LogsMessage)
		return
	}

	referencesNonExistentSecret := secretref.ReferencesNonExistentSecret(ctx, r.Client, pipeline)
	if referencesNonExistentSecret {
		conditions.SetPendingCondition(ctx, &pipeline.Status.Conditions, pipeline.Generation, conditions.ReasonReferencedSecretMissing, pipeline.Name, conditions.LogsMessage)
		return
	}

	fluentBitReady, err := r.prober.IsReady(ctx, r.config.DaemonSet)
	if err != nil {
		logf.FromContext(ctx).V(1).Error(err, "Failed to probe fluent bit daemonset")
		fluentBitReady = false
	}

	if !fluentBitReady {
		conditions.SetPendingCondition(ctx, &pipeline.Status.Conditions, pipeline.Generation, conditions.ReasonFluentBitDSNotReady, pipeline.Name, conditions.LogsMessage)
		return
	}

	conditions.SetRunningCondition(ctx, &pipeline.Status.Conditions, pipeline.Generation, conditions.ReasonFluentBitDSReady, pipeline.Name, conditions.LogsMessage)
}
