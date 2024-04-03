package logpipeline

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
)

const twoWeeks = time.Hour * 24 * 7 * 2

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
	status := metav1.ConditionTrue
	reason := conditions.ReasonConfigurationGenerated
	certValidationResult := getTLSCertValidationResult(ctx, pipeline, r.tlsCertValidator, r.Client)
	if secretref.ReferencesNonExistentSecret(ctx, r.Client, pipeline) {
		status = metav1.ConditionFalse
		reason = conditions.ReasonReferencedSecretMissing
	}

	if pipeline.Spec.Output.IsLokiDefined() {
		status = metav1.ConditionFalse
		reason = conditions.ReasonUnsupportedLokiOutput
	}

	message := conditions.MessageForLogPipeline(reason)

	if !certValidationResult.CertValid {
		status = metav1.ConditionFalse
		reason = conditions.ReasonTLSCertificateInvalid
		message = fmt.Sprintf(conditions.MessageForLogPipeline(reason), certValidationResult.CertValidationMessage)
	}

	if !certValidationResult.PrivateKeyValid {
		status = metav1.ConditionFalse
		reason = conditions.ReasonTLSPrivateKeyInvalid
		message = fmt.Sprintf(conditions.MessageForLogPipeline(reason), certValidationResult.PrivateKeyValidationMessage)
	}

	if time.Now().After(certValidationResult.Validity) {
		status = metav1.ConditionFalse
		reason = conditions.ReasonTLSCertificateExpired
	}

	//ensure not expired and about to expire
	validUntil := time.Until(certValidationResult.Validity)
	if validUntil > 0 && validUntil <= twoWeeks {
		status = metav1.ConditionTrue
		reason = conditions.ReasonTLSCertificateAboutToExpire
	}

	if reason == conditions.ReasonTLSCertificateAboutToExpire || reason == conditions.ReasonTLSCertificateExpired {
		message = fmt.Sprintf(message, certValidationResult.Validity.Format(time.DateOnly))
	}
	condition := metav1.Condition{
		Type:               conditions.TypeConfigurationGenerated,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: pipeline.Generation,
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, condition)
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
