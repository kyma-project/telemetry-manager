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

type DaemonsetProber interface {
	IsReady(ctx context.Context, name types.NamespacedName) error
}

type ErrorToMessageConverter interface {
	Convert(err error) string
}

func GetGatewayHealthyCondition(ctx context.Context, prober DeploymentProber, namespacedName types.NamespacedName, errToMsgCon ErrorToMessageConverter, signalType string) *metav1.Condition {
	status := metav1.ConditionTrue
	reason := conditions.ReasonGatewayReady
	msg := conditions.MessageForTracePipeline(reason)

	if signalType == SignalTypeMetrics {
		msg = conditions.MessageForMetricPipeline(reason)
	}

	err := prober.IsReady(ctx, namespacedName)
	if err != nil && !workloadstatus.IsRolloutInProgressError(err) {
		logf.FromContext(ctx).V(1).Error(err, "Failed to probe trace gateway - set condition as not healthy")
		status = metav1.ConditionFalse
		reason = conditions.ReasonGatewayNotReady
		msg = errToMsgCon.Convert(err)
	}

	if workloadstatus.IsRolloutInProgressError(err) {
		status = metav1.ConditionTrue
		reason = conditions.ReasonGatewayReady
		msg = errToMsgCon.Convert(err)
	}

	return &metav1.Condition{
		Type:    conditions.TypeGatewayHealthy,
		Status:  status,
		Reason:  reason,
		Message: msg,
	}

}

func GetAgentHealthyCondition(ctx context.Context, prober DaemonsetProber, namespacedName types.NamespacedName, errToMsgCon ErrorToMessageConverter, signalType string) *metav1.Condition {
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
		reason = conditions.ReasonAgentReady
		msg = errToMsgCon.Convert(err)
	}

	return &metav1.Condition{
		Type:    conditions.TypeAgentHealthy,
		Status:  status,
		Reason:  reason,
		Message: msg,
	}
}
