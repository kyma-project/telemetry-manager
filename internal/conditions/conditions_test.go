package conditions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMessageFor(t *testing.T) {
	t.Run("should return correct message which is common to all pipelines", func(t *testing.T) {
		message := MessageFor(ReasonReferencedSecretMissing, LogsMessage)
		require.Equal(t, commonMessage[ReasonReferencedSecretMissing], message)
	})

	t.Run("should return correct message which is unique to each pipeline", func(t *testing.T) {
		logsDaemonSetNotReadyMessage := MessageFor(ReasonDaemonSetNotReady, LogsMessage)
		require.Equal(t, LogsMessage[ReasonDaemonSetNotReady], logsDaemonSetNotReadyMessage)

		tracesDeploymentNotReadyMessage := MessageFor(ReasonDeploymentNotReady, TracesMessage)
		require.Equal(t, TracesMessage[ReasonDeploymentNotReady], tracesDeploymentNotReadyMessage)

		metricsDeploymentNotReadyMessage := MessageFor(ReasonDeploymentNotReady, MetricsMessage)
		require.Equal(t, MetricsMessage[ReasonDeploymentNotReady], metricsDeploymentNotReadyMessage)
	})

	t.Run("should return empty message for reasons which do not have a dedicated message", func(t *testing.T) {
		metricsAgentNotRequiredMessage := MessageFor(ReasonMetricAgentNotRequired, MetricsMessage)
		require.Equal(t, "", metricsAgentNotRequiredMessage)
	})
}

func TestHandlePendingCondition(t *testing.T) {
	t.Run("should just set pending condition to true if running condition is not in the conditions list", func(t *testing.T) {
		conditions := []metav1.Condition{
			{
				Type:               TypeAgentHealthy,
				Status:             metav1.ConditionFalse,
				Reason:             ReasonDaemonSetNotReady,
				Message:            MessageFor(ReasonDaemonSetNotReady, LogsMessage),
				LastTransitionTime: metav1.Now(),
			},
			{
				Type:               TypeConfigurationGenerated,
				Status:             metav1.ConditionTrue,
				Reason:             ReasonConfigurationGenerated,
				Message:            MessageFor(ReasonConfigurationGenerated, LogsMessage),
				LastTransitionTime: metav1.Now(),
			},
		}
		generation := int64(1)
		reason := ReasonFluentBitDSNotReady

		HandlePendingCondition(context.Background(), &conditions, generation, reason, "pipeline", LogsMessage)

		conditionsSize := len(conditions)
		pendingCond := conditions[conditionsSize-1]
		require.Equal(t, TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionTrue, pendingCond.Status)
		require.Equal(t, reason, pendingCond.Reason)
		pendingCondMsg := PendingTypeDeprecationMsg + MessageFor(reason, LogsMessage)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)
	})

	t.Run("should remove running condition and set pending condition to true", func(t *testing.T) {
		conditions := []metav1.Condition{
			{
				Type:               TypeAgentHealthy,
				Status:             metav1.ConditionFalse,
				Reason:             ReasonDaemonSetNotReady,
				Message:            MessageFor(ReasonDaemonSetNotReady, LogsMessage),
				LastTransitionTime: metav1.Now(),
			},
			{
				Type:               TypeConfigurationGenerated,
				Status:             metav1.ConditionTrue,
				Reason:             ReasonConfigurationGenerated,
				Message:            MessageFor(ReasonConfigurationGenerated, LogsMessage),
				LastTransitionTime: metav1.Now(),
			},
			{
				Type:               TypePending,
				Status:             metav1.ConditionFalse,
				Reason:             ReasonFluentBitDSNotReady,
				Message:            PendingTypeDeprecationMsg + MessageFor(ReasonFluentBitDSNotReady, LogsMessage),
				LastTransitionTime: metav1.Now(),
			},
			{
				Type:               TypeRunning,
				Status:             metav1.ConditionTrue,
				Reason:             ReasonFluentBitDSReady,
				Message:            RunningTypeDeprecationMsg + MessageFor(ReasonFluentBitDSReady, LogsMessage),
				LastTransitionTime: metav1.Now(),
			},
		}
		generation := int64(1)
		reason := ReasonFluentBitDSNotReady

		HandlePendingCondition(context.Background(), &conditions, generation, reason, "pipeline", LogsMessage)

		runningCond := meta.FindStatusCondition(conditions, TypeRunning)
		require.Nil(t, runningCond)

		conditionsSize := len(conditions)
		pendingCond := conditions[conditionsSize-1]
		require.Equal(t, TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionTrue, pendingCond.Status)
		require.Equal(t, reason, pendingCond.Reason)
		pendingCondMsg := PendingTypeDeprecationMsg + MessageFor(reason, LogsMessage)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)
	})
}

func TestHandleRunningCondition(t *testing.T) {
	t.Run("should set pending condition to false and set running condition to true", func(t *testing.T) {
		conditions := []metav1.Condition{
			{
				Type:               TypeAgentHealthy,
				Status:             metav1.ConditionTrue,
				Reason:             ReasonDaemonSetReady,
				Message:            MessageFor(ReasonDaemonSetReady, LogsMessage),
				LastTransitionTime: metav1.Now(),
			},
			{
				Type:               TypeConfigurationGenerated,
				Status:             metav1.ConditionTrue,
				Reason:             ReasonConfigurationGenerated,
				Message:            MessageFor(ReasonConfigurationGenerated, LogsMessage),
				LastTransitionTime: metav1.Now(),
			},
		}
		generation := int64(1)
		runningReason := ReasonFluentBitDSReady
		pendingReason := ReasonFluentBitDSNotReady

		HandleRunningCondition(context.Background(), &conditions, generation, runningReason, pendingReason, "pipeline", LogsMessage)

		conditionsSize := len(conditions)

		pendingCond := conditions[conditionsSize-2]
		require.Equal(t, TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionFalse, pendingCond.Status)
		require.Equal(t, pendingReason, pendingCond.Reason)
		pendingCondMsg := PendingTypeDeprecationMsg + MessageFor(pendingReason, LogsMessage)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)

		runningCond := conditions[conditionsSize-1]
		require.Equal(t, TypeRunning, runningCond.Type)
		require.Equal(t, metav1.ConditionTrue, runningCond.Status)
		require.Equal(t, runningReason, runningCond.Reason)
		runningCondMsg := RunningTypeDeprecationMsg + MessageFor(runningReason, LogsMessage)
		require.Equal(t, runningCondMsg, runningCond.Message)
		require.Equal(t, generation, runningCond.ObservedGeneration)
		require.NotEmpty(t, runningCond.LastTransitionTime)
	})
}
