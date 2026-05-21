package fluentbit

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/errortypes"
	"github.com/kyma-project/telemetry-manager/internal/pipelines"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
	"github.com/kyma-project/telemetry-manager/internal/validators/endpoint"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

func (r *Reconciler) updateStatus(ctx context.Context, pipelineName string) (flowHealthProbingFailed bool, err error) {
	var pipeline telemetryv1beta1.LogPipeline
	if err := r.Get(ctx, types.NamespacedName{Name: pipelineName}, &pipeline); err != nil {
		if apierrors.IsNotFound(err) {
			logf.FromContext(ctx).V(1).Info("Skipping status update for LogPipeline - not found")
			return false, nil
		}

		return false, fmt.Errorf("failed to get LogPipeline: %w", err)
	}

	if pipeline.DeletionTimestamp != nil {
		logf.FromContext(ctx).V(1).Info("Skipping status update for LogPipeline - marked for deletion")
		return false, nil
	}

	var allErrors error = nil

	if err := r.updateStatusUnsupportedMode(ctx, &pipeline); err != nil {
		allErrors = errors.Join(allErrors, err)
	}

	r.setAgentHealthyCondition(ctx, &pipeline)
	r.setFluentBitConfigGeneratedCondition(ctx, &pipeline)

	if err := r.setFlowHealthCondition(ctx, &pipeline); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to set flow health condition")
		flowHealthProbingFailed = true
	}

	if err := r.Status().Update(ctx, &pipeline); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to update LogPipeline status: %w", err))
	}

	return flowHealthProbingFailed, allErrors
}

func (r *Reconciler) updateStatusUnsupportedMode(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline) error {
	desiredUnsupportedMode := logpipelineutils.ContainsCustomPlugin(pipeline)
	if pipeline.Status.UnsupportedMode == nil || *pipeline.Status.UnsupportedMode != desiredUnsupportedMode {
		pipeline.Status.UnsupportedMode = &desiredUnsupportedMode
		if err := r.Status().Update(ctx, pipeline); err != nil {
			return fmt.Errorf("failed to update LogPipeline unsupported mode status: %w", err)
		}
	}

	return nil
}

func (r *Reconciler) setAgentHealthyCondition(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline) {
	condition := commonstatus.GetAgentHealthyCondition(ctx,
		r.agentProber,
		types.NamespacedName{Name: names.FluentBit, Namespace: r.globals.TargetNamespace()},
		r.errToMsgConverter,
		pipelines.SignalTypeLogFluentBit)
	meta.SetStatusCondition(&pipeline.Status.Conditions, *condition)
}

func (r *Reconciler) setFluentBitConfigGeneratedCondition(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline) {
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

func (r *Reconciler) evaluateConfigGeneratedCondition(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline) (status metav1.ConditionStatus, reason string, message string) {
	if r.globals.OperateInFIPSMode() {
		return metav1.ConditionFalse, conditions.ReasonNoFluentbitInFipsMode, conditions.MessageForFluentBitLogPipeline(conditions.ReasonNoFluentbitInFipsMode)
	}

	err := r.pipelineValidator.Validate(ctx, pipeline)
	if err == nil {
		return metav1.ConditionTrue, conditions.ReasonAgentConfigured, conditions.MessageForFluentBitLogPipeline(conditions.ReasonAgentConfigured)
	}

	if errors.Is(err, resourcelock.ErrMaxPipelinesExceeded) {
		return metav1.ConditionFalse, conditions.ReasonMaxPipelinesExceeded, conditions.ConvertErrToMsg(err)
	}

	if errors.Is(err, secretref.ErrSecretRefNotFound) || errors.Is(err, secretref.ErrSecretKeyNotFound) || errors.Is(err, secretref.ErrSecretRefMissingFields) {
		return metav1.ConditionFalse, conditions.ReasonReferencedSecretMissing, conditions.ConvertErrToMsg(err)
	}

	if endpoint.IsEndpointInvalidError(err) {
		return metav1.ConditionFalse,
			conditions.ReasonEndpointInvalid,
			fmt.Sprintf(conditions.MessageForFluentBitLogPipeline(conditions.ReasonEndpointInvalid), err.Error())
	}

	if APIRequestFailed, _ := errors.AsType[*errortypes.APIRequestFailedError](err); APIRequestFailed != nil {
		return metav1.ConditionFalse, conditions.ReasonValidationFailed, conditions.MessageForFluentBitLogPipeline(conditions.ReasonValidationFailed)
	}

	return conditions.EvaluateTLSCertCondition(err)
}

func (r *Reconciler) setFlowHealthCondition(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline) error {
	status, reason, err := r.evaluateFlowHealthCondition(ctx, pipeline)

	condition := metav1.Condition{
		Type:               conditions.TypeFlowHealthy,
		Status:             status,
		Reason:             reason,
		Message:            conditions.MessageForFluentBitLogPipeline(reason),
		ObservedGeneration: pipeline.Generation,
	}

	meta.SetStatusCondition(&pipeline.Status.Conditions, condition)

	return err
}

func (r *Reconciler) evaluateFlowHealthCondition(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline) (metav1.ConditionStatus, string, error) {
	configGeneratedStatus, _, _ := r.evaluateConfigGeneratedCondition(ctx, pipeline)
	if configGeneratedStatus == metav1.ConditionFalse {
		return metav1.ConditionFalse, conditions.ReasonSelfMonConfigNotGenerated, nil
	}

	// Check if self-monitor deployment is ready before attempting to probe
	if r.selfMonitorProber != nil {
		if err := r.selfMonitorProber.IsReady(ctx, types.NamespacedName{
			Name:      names.SelfMonitor,
			Namespace: r.globals.TargetNamespace(),
		}); err != nil {
			logf.FromContext(ctx).V(1).Info("Self-monitor not ready, skipping flow health probe", "error", err)
			// Only return error if self-monitor service endpoint exists but probing failed
			// If self-monitor is not deployed (ErrDeploymentNotFound), don't return error to avoid unnecessary requeue
			if !errors.Is(err, workloadstatus.ErrDeploymentNotFound) {
				return metav1.ConditionUnknown, conditions.ReasonSelfMonAgentProbingFailed, fmt.Errorf("self-monitor deployment not ready: %w", err)
			}
			return metav1.ConditionUnknown, conditions.ReasonSelfMonAgentProbingFailed, nil
		}
	}

	probeResult, err := r.flowHealthProber.Probe(ctx, pipeline.Name)
	if err != nil {
		return metav1.ConditionUnknown, conditions.ReasonSelfMonAgentProbingFailed, fmt.Errorf("failed to probe flow health: %w", err)
	}

	logf.FromContext(ctx).V(1).Info("Probed flow health", "result", probeResult)

	reason := flowHealthReasonFor(probeResult)
	if probeResult.Healthy {
		return metav1.ConditionTrue, reason, nil
	}

	return metav1.ConditionFalse, reason, nil
}

func flowHealthReasonFor(probeResult prober.FluentBitProbeResult) string {
	switch {
	case probeResult.AllDataDropped:
		return conditions.ReasonSelfMonAgentAllDataDropped
	case probeResult.SomeDataDropped:
		return conditions.ReasonSelfMonAgentSomeDataDropped
	case probeResult.NoLogsDelivered:
		return conditions.ReasonSelfMonAgentNoLogsDelivered
	case probeResult.BufferFillingUp:
		return conditions.ReasonSelfMonAgentBufferFillingUp
	default:
		return conditions.ReasonSelfMonFlowHealthy
	}
}
