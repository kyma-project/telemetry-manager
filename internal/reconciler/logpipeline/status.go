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
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
)

func (r *Reconciler) updateStatus(ctx context.Context, pipelineName string) error {
	var pipeline telemetryv1alpha1.LogPipeline
	if err := r.Get(ctx, types.NamespacedName{Name: pipelineName}, &pipeline); err != nil {
		if apierrors.IsNotFound(err) {
			logf.FromContext(ctx).V(1).Info("Skipping status update for LogPipeline - not found")
			return nil
		}

		return fmt.Errorf("failed to get LogPipeline: %w", err)
	}

	if pipeline.DeletionTimestamp != nil {
		logf.FromContext(ctx).V(1).Info("Skipping status update for LogPipeline - marked for deletion")
		return nil
	}

	if err := r.updateStatusUnsupportedMode(ctx, &pipeline); err != nil {
		return err
	}

	r.setAgentHealthyCondition(ctx, &pipeline)
	r.setFluentBitConfigGeneratedCondition(ctx, &pipeline)

	if r.flowHealthProbingEnabled {
		r.setFlowHealthCondition(ctx, &pipeline)
	}

	r.setLegacyConditions(ctx, &pipeline)

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
			return fmt.Errorf("failed to update LogPipeline unsupported mode status: %w", err)
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

	condition := metav1.Condition{
		Type:               conditions.TypeAgentHealthy,
		Status:             status,
		Reason:             reason,
		Message:            conditions.MessageForLogPipeline(reason),
		ObservedGeneration: pipeline.Generation,
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, condition)
}

func (r *Reconciler) setFluentBitConfigGeneratedCondition(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) {
	status, reason, message := r.evaluateConfigGeneratedCondition(ctx, pipeline)

	condition := metav1.Condition{
		Type:               conditions.TypeConfigurationGenerated,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: pipeline.Generation,
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, condition)
}

func (r *Reconciler) evaluateConfigGeneratedCondition(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) (status metav1.ConditionStatus, reason string, message string) {
	if pipeline.Spec.Output.IsLokiDefined() {
		return metav1.ConditionFalse, conditions.ReasonUnsupportedLokiOutput, conditions.MessageForLogPipeline(conditions.ReasonUnsupportedLokiOutput)
	}

	if secretref.ReferencesNonExistentSecret(ctx, r.Client, pipeline) {
		return metav1.ConditionFalse, conditions.ReasonReferencedSecretMissing, conditions.MessageForMetricPipeline(conditions.ReasonReferencedSecretMissing)
	}

	if tlsCertValidationRequired(pipeline) {
		cert := pipeline.Spec.Output.HTTP.TLSConfig.Cert
		key := pipeline.Spec.Output.HTTP.TLSConfig.Key

		err := r.tlsCertValidator.ValidateCertificate(ctx, cert, key)
		return conditions.EvaluateTLSCertCondition(err)
	}

	return metav1.ConditionTrue, conditions.ReasonConfigurationGenerated, conditions.MessageForMetricPipeline(conditions.ReasonConfigurationGenerated)
}

func (r *Reconciler) setFlowHealthCondition(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) {
	var reason string
	var status metav1.ConditionStatus

	probeResult, err := r.flowHealthProber.Probe(ctx, pipeline.Name)
	if err == nil {
		logf.FromContext(ctx).V(1).Info("Probed flow health", "result", probeResult)

		reason = flowHealthReasonFor(probeResult)
		if probeResult.Healthy {
			status = metav1.ConditionTrue
		} else {
			status = metav1.ConditionFalse
		}
	} else {
		logf.FromContext(ctx).Error(err, "Failed to probe flow health")

		reason = conditions.ReasonFlowHealthy
		status = metav1.ConditionUnknown
	}

	condition := metav1.Condition{
		Type:               conditions.TypeFlowHealthy,
		Status:             status,
		Reason:             reason,
		Message:            conditions.MessageForLogPipeline(reason),
		ObservedGeneration: pipeline.Generation,
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, condition)
}

func flowHealthReasonFor(probeResult prober.LogPipelineProbeResult) string {
	switch {
	case probeResult.AllDataDropped:
		return conditions.ReasonAllDataDropped
	case probeResult.SomeDataDropped:
		return conditions.ReasonSomeDataDropped
	case probeResult.NoLogsDelivered:
		return conditions.ReasonNoLogsDelivered
	case probeResult.BufferFillingUp:
		return conditions.ReasonBufferFillingUp
	default:
		return conditions.ReasonFlowHealthy
	}
}

func (r *Reconciler) setLegacyConditions(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) {
	if pipeline.Spec.Output.IsLokiDefined() {
		conditions.HandlePendingCondition(&pipeline.Status.Conditions, pipeline.Generation,
			conditions.ReasonUnsupportedLokiOutput,
			conditions.MessageForLogPipeline(conditions.ReasonUnsupportedLokiOutput))
		return
	}

	referencesNonExistentSecret := secretref.ReferencesNonExistentSecret(ctx, r.Client, pipeline)
	if referencesNonExistentSecret {
		conditions.HandlePendingCondition(&pipeline.Status.Conditions, pipeline.Generation,
			conditions.ReasonReferencedSecretMissing,
			conditions.MessageForLogPipeline(conditions.ReasonReferencedSecretMissing))
		return
	}

	fluentBitReady, err := r.prober.IsReady(ctx, r.config.DaemonSet)
	if err != nil {
		logf.FromContext(ctx).V(1).Error(err, "Failed to probe fluent bit daemonset")
		fluentBitReady = false
	}

	if !fluentBitReady {
		conditions.HandlePendingCondition(&pipeline.Status.Conditions, pipeline.Generation,
			conditions.ReasonFluentBitDSNotReady,
			conditions.MessageForLogPipeline(conditions.ReasonFluentBitDSNotReady))
		return
	}

	conditions.HandleRunningCondition(&pipeline.Status.Conditions, pipeline.Generation,
		conditions.ReasonFluentBitDSReady,
		conditions.ReasonFluentBitDSNotReady,
		conditions.MessageForLogPipeline(conditions.ReasonFluentBitDSReady),
		conditions.MessageForLogPipeline(conditions.ReasonFluentBitDSNotReady))
}
