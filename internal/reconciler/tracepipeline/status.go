package tracepipeline

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

func (r *Reconciler) updateStatus(ctx context.Context, pipelineName string, withinPipelineCountLimit bool) error {
	var pipeline telemetryv1alpha1.TracePipeline
	if err := r.Get(ctx, types.NamespacedName{Name: pipelineName}, &pipeline); err != nil {
		if apierrors.IsNotFound(err) {
			logf.FromContext(ctx).V(1).Info("Skipping status update for TracePipeline - not found")
			return nil
		}

		return fmt.Errorf("failed to get TracePipeline: %v", err)
	}

	if pipeline.DeletionTimestamp != nil {
		logf.FromContext(ctx).V(1).Info("Skipping status update for TracePipeline - marked for deletion")
		return nil
	}

	// If the "GatewayHealthy" type doesn't exist in the conditions
	// then we need to reset the conditions list to ensure that the "Pending" and "Running" conditions are appended to the end of the conditions list
	// Check step 3 in https://github.com/kyma-project/telemetry-manager/blob/main/docs/contributor/arch/004-consolidate-pipeline-statuses.md#decision
	if meta.FindStatusCondition(pipeline.Status.Conditions, conditions.TypeGatewayHealthy) == nil {
		pipeline.Status.Conditions = []metav1.Condition{}
	}

	r.setGatewayHealthyCondition(ctx, &pipeline)
	r.setGatewayConfigGeneratedCondition(ctx, &pipeline, withinPipelineCountLimit)
	if r.flowHealthProbingEnabled {
		r.setFlowHealthCondition(ctx, &pipeline)
	}
	r.setLegacyConditions(ctx, &pipeline, withinPipelineCountLimit)

	if err := r.Status().Update(ctx, &pipeline); err != nil {
		return fmt.Errorf("failed to update TracePipeline status: %w", err)
	}

	return nil
}

func (r *Reconciler) setGatewayHealthyCondition(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline) {
	healthy, err := r.prober.IsReady(ctx, types.NamespacedName{Name: r.config.Gateway.BaseName, Namespace: r.config.Gateway.Namespace})
	if err != nil {
		logf.FromContext(ctx).V(1).Error(err, "Failed to probe trace gateway - set condition as not healthy")
		healthy = false
	}

	status := metav1.ConditionFalse
	reason := conditions.ReasonDeploymentNotReady
	if healthy {
		status = metav1.ConditionTrue
		reason = conditions.ReasonDeploymentReady
	}

	condition := metav1.Condition{
		Type:               conditions.TypeGatewayHealthy,
		Status:             status,
		Reason:             reason,
		Message:            conditions.MessageForTracePipeline(reason),
		ObservedGeneration: pipeline.Generation,
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, condition)
}

func (r *Reconciler) setGatewayConfigGeneratedCondition(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline, withinPipelineCountLimit bool) {
	status := metav1.ConditionTrue
	reason := conditions.ReasonConfigurationGenerated
	// We will add some extra information to message field in conditions.EvaluateTLSCertCondition.
	// So assign message in each condition
	message := conditions.MessageForLogPipeline(reason)

	if secretref.ReferencesNonExistentSecret(ctx, r.Client, pipeline) {
		status = metav1.ConditionFalse
		reason = conditions.ReasonReferencedSecretMissing
		message = conditions.MessageForTracePipeline(reason)
	}

	if !secretref.ReferencesNonExistentSecret(ctx, r.Client, pipeline) && pipeline.TLSCertAndKeyExist() {
		// we should set TLSCert status only if tls cert is present
		certValidationResult := getTLSCertValidationResult(ctx, pipeline, r.tlsCertValidator)
		status, reason, message = conditions.EvaluateTLSCertCondition(certValidationResult)
	}

	if !withinPipelineCountLimit {
		status = metav1.ConditionFalse
		reason = conditions.ReasonMaxPipelinesExceeded
		message = conditions.MessageForTracePipeline(reason)
	}

	condition := metav1.Condition{
		Type:               conditions.TypeConfigurationGenerated,
		Status:             status,
		ObservedGeneration: pipeline.Generation,
		Reason:             reason,
		Message:            message,
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, condition)
}

func (r *Reconciler) setFlowHealthCondition(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline) {
	var reason string
	var status metav1.ConditionStatus

	probeResult, err := r.flowHealthProber.Probe(ctx, pipeline.Name)
	if err == nil {
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
		Message:            conditions.MessageForTracePipeline(reason),
		ObservedGeneration: pipeline.Generation,
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, condition)
}

func flowHealthReasonFor(probeResult prober.ProbeResult) string {
	if probeResult.AllDataDropped {
		return conditions.ReasonAllDataDropped
	}
	if probeResult.SomeDataDropped {
		return conditions.ReasonSomeDataDropped
	}
	if probeResult.QueueAlmostFull {
		return conditions.ReasonBufferFillingUp
	}
	if probeResult.Throttling {
		return conditions.ReasonGatewayThrottling
	}
	return conditions.ReasonFlowHealthy
}

func (r *Reconciler) setLegacyConditions(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline, withinPipelineCountLimit bool) {
	if !withinPipelineCountLimit {
		conditions.HandlePendingCondition(&pipeline.Status.Conditions, pipeline.Generation,
			conditions.ReasonMaxPipelinesExceeded,
			conditions.MessageForTracePipeline(conditions.ReasonMaxPipelinesExceeded))
		return
	}

	referencesNonExistentSecret := secretref.ReferencesNonExistentSecret(ctx, r.Client, pipeline)
	if referencesNonExistentSecret {
		conditions.HandlePendingCondition(&pipeline.Status.Conditions, pipeline.Generation,
			conditions.ReasonReferencedSecretMissing,
			conditions.MessageForTracePipeline(conditions.ReasonReferencedSecretMissing))
		return
	}

	gatewayReady, err := r.prober.IsReady(ctx, types.NamespacedName{Name: r.config.Gateway.BaseName, Namespace: r.config.Gateway.Namespace})
	if err != nil {
		logf.FromContext(ctx).V(1).Error(err, "Failed to probe trace gateway")
		gatewayReady = false
	}

	if !gatewayReady {
		conditions.HandlePendingCondition(&pipeline.Status.Conditions, pipeline.Generation,
			conditions.ReasonTraceGatewayDeploymentNotReady,
			conditions.MessageForTracePipeline(conditions.ReasonTraceGatewayDeploymentNotReady))
		return
	}

	conditions.HandleRunningCondition(&pipeline.Status.Conditions, pipeline.Generation,
		conditions.ReasonTraceGatewayDeploymentReady,
		conditions.ReasonTraceGatewayDeploymentNotReady,
		conditions.MessageForTracePipeline(conditions.ReasonTraceGatewayDeploymentReady),
		conditions.MessageForTracePipeline(conditions.ReasonTraceGatewayDeploymentNotReady))
}
