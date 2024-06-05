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
	"github.com/kyma-project/telemetry-manager/internal/tlscert"
)

func (r *Reconciler) updateStatus(ctx context.Context, pipelineName string, withinPipelineCountLimit bool) error {
	var pipeline telemetryv1alpha1.TracePipeline
	if err := r.Get(ctx, types.NamespacedName{Name: pipelineName}, &pipeline); err != nil {
		if apierrors.IsNotFound(err) {
			logf.FromContext(ctx).V(1).Info("Skipping status update for TracePipeline - not found")
			return nil
		}

		return fmt.Errorf("failed to get TracePipeline: %w", err)
	}

	if pipeline.DeletionTimestamp != nil {
		logf.FromContext(ctx).V(1).Info("Skipping status update for TracePipeline - marked for deletion")
		return nil
	}

	r.setGatewayHealthyCondition(ctx, &pipeline)
	r.setGatewayConfigGeneratedCondition(ctx, &pipeline, withinPipelineCountLimit)
	r.setFlowHealthCondition(ctx, &pipeline)
	r.setLegacyConditions(ctx, &pipeline)

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
	reason := conditions.ReasonGatewayNotReady
	if healthy {
		status = metav1.ConditionTrue
		reason = conditions.ReasonGatewayReady
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
	status, reason, message := r.evaluateConfigGeneratedCondition(ctx, pipeline, withinPipelineCountLimit)

	condition := metav1.Condition{
		Type:               conditions.TypeConfigurationGenerated,
		Status:             status,
		ObservedGeneration: pipeline.Generation,
		Reason:             reason,
		Message:            message,
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, condition)
}

func (r *Reconciler) evaluateConfigGeneratedCondition(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline, withinPipelineCountLimit bool) (status metav1.ConditionStatus, reason string, message string) {
	if !withinPipelineCountLimit {
		return metav1.ConditionFalse, conditions.ReasonMaxPipelinesExceeded, conditions.MessageForTracePipeline(conditions.ReasonMaxPipelinesExceeded)
	}

	if secretref.ReferencesNonExistentSecret(ctx, r.Client, pipeline) {
		return metav1.ConditionFalse, conditions.ReasonReferencedSecretMissing, conditions.MessageForTracePipeline(conditions.ReasonReferencedSecretMissing)
	}

	if tlsCertValidationRequired(pipeline) {
		if tlsCertValidationRequired(pipeline) {
			tlsConfig := tlscert.TLSBundle{
				Cert: pipeline.Spec.Output.Otlp.TLS.Cert,
				Key:  pipeline.Spec.Output.Otlp.TLS.Key,
				CA:   pipeline.Spec.Output.Otlp.TLS.CA,
			}

			err := r.tlsCertValidator.Validate(ctx, tlsConfig)
			return conditions.EvaluateTLSCertCondition(err, conditions.ReasonGatewayConfigured, conditions.MessageForTracePipeline(conditions.ReasonGatewayConfigured))
		}
	}

	return metav1.ConditionTrue, conditions.ReasonGatewayConfigured, conditions.MessageForTracePipeline(conditions.ReasonGatewayConfigured)
}

func (r *Reconciler) setFlowHealthCondition(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline) {
	status, reason := r.evaluateFlowHealthCondition(ctx, pipeline)

	condition := metav1.Condition{
		Type:               conditions.TypeFlowHealthy,
		Status:             status,
		Reason:             reason,
		Message:            conditions.MessageForTracePipeline(reason),
		ObservedGeneration: pipeline.Generation,
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, condition)
}

func (r *Reconciler) evaluateFlowHealthCondition(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline) (metav1.ConditionStatus, string) {
	if meta.IsStatusConditionFalse(pipeline.Status.Conditions, conditions.TypeConfigurationGenerated) {
		return metav1.ConditionFalse, conditions.ReasonSelfMonConfigNotGenerated
	}

	probeResult, err := r.flowHealthProber.Probe(ctx, pipeline.Name)
	if err != nil {
		logf.FromContext(ctx).Error(err, "Failed to probe flow health")
		return metav1.ConditionUnknown, conditions.ReasonSelfMonProbingFailed
	}

	logf.FromContext(ctx).V(1).Info("Probed flow health", "result", probeResult)

	reason := flowHealthReasonFor(probeResult)
	if probeResult.Healthy {
		return metav1.ConditionTrue, reason
	}

	return metav1.ConditionFalse, reason

}

func flowHealthReasonFor(probeResult prober.OTelPipelineProbeResult) string {
	if probeResult.AllDataDropped {
		return conditions.ReasonSelfMonAllDataDropped
	}
	if probeResult.SomeDataDropped {
		return conditions.ReasonSelfMonSomeDataDropped
	}
	if probeResult.QueueAlmostFull {
		return conditions.ReasonSelfMonBufferFillingUp
	}
	if probeResult.Throttling {
		return conditions.ReasonSelfMonGatewayThrottling
	}
	return conditions.ReasonSelfMonFlowHealthy
}

func (r *Reconciler) setLegacyConditions(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline) {
	if meta.IsStatusConditionFalse(pipeline.Status.Conditions, conditions.TypeConfigurationGenerated) {
		evaluatedCondition := meta.FindStatusCondition(pipeline.Status.Conditions, conditions.TypeConfigurationGenerated)
		conditions.HandlePendingCondition(&pipeline.Status.Conditions, pipeline.Generation,
			evaluatedCondition.Reason,
			evaluatedCondition.Message)
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
