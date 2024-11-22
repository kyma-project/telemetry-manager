package commonstatus

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

const (
	SignalTypeTraces  = "traces"
	SignalTypeMetrics = "metrics"
	SignalTypeLogs    = "logs"
)

type DeploymentProber interface {
	IsReady(ctx context.Context, name types.NamespacedName) error
}

type Prober interface {
	IsReady(ctx context.Context, name types.NamespacedName) error
}

type ErrorToMessageConverter interface {
	Convert(err error) string
}

//nolint:dupl // abstracting the common code will still have duplicates
func GetGatewayHealthyCondition(ctx context.Context, prober Prober, namespacedName types.NamespacedName, errToMsgCon ErrorToMessageConverter, signalType string) *metav1.Condition {
	status := metav1.ConditionTrue
	reason := conditions.ReasonGatewayReady
	msg := conditions.MessageForTracePipeline(reason)

	if signalType == SignalTypeMetrics {
		msg = conditions.MessageForMetricPipeline(reason)
	}

	if signalType == SignalTypeLogs {
		msg = conditions.MessageForLogPipeline(reason)
	}

	err := prober.IsReady(ctx, namespacedName)
	if err != nil && !workloadstatus.IsRolloutInProgressError(err) {
		logf.FromContext(ctx).V(1).Error(err, "Failed to probe gateway - set condition as not healthy")

		status = metav1.ConditionFalse
		reason = conditions.ReasonGatewayNotReady
		msg = errToMsgCon.Convert(err)
	}

	if workloadstatus.IsRolloutInProgressError(err) {
		status = metav1.ConditionTrue
		reason = conditions.ReasonRolloutInProgress
		msg = errToMsgCon.Convert(err)
	}

	return &metav1.Condition{
		Type:    conditions.TypeGatewayHealthy,
		Status:  status,
		Reason:  reason,
		Message: msg,
	}
}

//nolint:dupl // abstracting the common code will still have duplicates and would complicate the code.
func GetAgentHealthyCondition(ctx context.Context, prober Prober, namespacedName types.NamespacedName, errToMsgCon ErrorToMessageConverter, signalType string) *metav1.Condition {
	status := metav1.ConditionTrue
	reason := conditions.ReasonAgentReady
	msg := conditions.MessageForLogPipeline(reason)

	if signalType == SignalTypeMetrics {
		msg = conditions.MessageForMetricPipeline(reason)
	}

	err := prober.IsReady(ctx, namespacedName)
	if err != nil && !workloadstatus.IsRolloutInProgressError(err) {
		logf.FromContext(ctx).V(1).Error(err, "Failed to probe agent - set condition as not healthy")

		status = metav1.ConditionFalse
		reason = conditions.ReasonAgentNotReady
		msg = errToMsgCon.Convert(err)
	}

	if workloadstatus.IsRolloutInProgressError(err) {
		status = metav1.ConditionTrue
		reason = conditions.ReasonRolloutInProgress
		msg = errToMsgCon.Convert(err)
	}

	return &metav1.Condition{
		Type:    conditions.TypeAgentHealthy,
		Status:  status,
		Reason:  reason,
		Message: msg,
	}
}
