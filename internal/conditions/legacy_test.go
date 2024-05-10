package conditions

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHandlePendingCondition(t *testing.T) {
	t.Run("should just set pending condition to true if running condition is not in the conditions list", func(t *testing.T) {
		conditions := []metav1.Condition{
			{
				Type:               TypeAgentHealthy,
				Status:             metav1.ConditionFalse,
				Reason:             ReasonDaemonSetNotReady,
				Message:            MessageForLogPipeline(ReasonDaemonSetNotReady),
				LastTransitionTime: metav1.Now(),
			},
			{
				Type:               TypeConfigurationGenerated,
				Status:             metav1.ConditionTrue,
				Reason:             ReasonConfigurationGenerated,
				Message:            MessageForLogPipeline(ReasonConfigurationGenerated),
				LastTransitionTime: metav1.Now(),
			},
		}
		generation := int64(1)
		reason := ReasonFluentBitDSNotReady

		HandlePendingCondition(&conditions, generation, reason, MessageForLogPipeline(reason))

		conditionsSize := len(conditions)
		pendingCond := conditions[conditionsSize-1]
		require.Equal(t, TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionTrue, pendingCond.Status)
		require.Equal(t, reason, pendingCond.Reason)
		pendingCondMsg := PendingTypeDeprecationMsg + MessageForLogPipeline(reason)
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
				Message:            MessageForLogPipeline(ReasonDaemonSetNotReady),
				LastTransitionTime: metav1.Now(),
			},
			{
				Type:               TypeConfigurationGenerated,
				Status:             metav1.ConditionTrue,
				Reason:             ReasonConfigurationGenerated,
				Message:            MessageForLogPipeline(ReasonConfigurationGenerated),
				LastTransitionTime: metav1.Now(),
			},
			{
				Type:               TypePending,
				Status:             metav1.ConditionFalse,
				Reason:             ReasonFluentBitDSNotReady,
				Message:            PendingTypeDeprecationMsg + MessageForLogPipeline(ReasonFluentBitDSNotReady),
				LastTransitionTime: metav1.Now(),
			},
			{
				Type:               TypeRunning,
				Status:             metav1.ConditionTrue,
				Reason:             ReasonFluentBitDSReady,
				Message:            RunningTypeDeprecationMsg + MessageForLogPipeline(ReasonFluentBitDSReady),
				LastTransitionTime: metav1.Now(),
			},
		}
		generation := int64(1)
		reason := ReasonFluentBitDSNotReady

		HandlePendingCondition(&conditions, generation, reason, MessageForLogPipeline(reason))

		runningCond := meta.FindStatusCondition(conditions, TypeRunning)
		require.Nil(t, runningCond)

		conditionsSize := len(conditions)
		pendingCond := conditions[conditionsSize-1]
		require.Equal(t, TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionTrue, pendingCond.Status)
		require.Equal(t, reason, pendingCond.Reason)
		pendingCondMsg := PendingTypeDeprecationMsg + MessageForLogPipeline(reason)
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
				Message:            MessageForLogPipeline(ReasonDaemonSetReady),
				LastTransitionTime: metav1.Now(),
			},
			{
				Type:               TypeConfigurationGenerated,
				Status:             metav1.ConditionTrue,
				Reason:             ReasonConfigurationGenerated,
				Message:            MessageForLogPipeline(ReasonConfigurationGenerated),
				LastTransitionTime: metav1.Now(),
			},
		}
		generation := int64(1)
		runningReason := ReasonFluentBitDSReady
		pendingReason := ReasonFluentBitDSNotReady

		HandleRunningCondition(&conditions, generation, runningReason, pendingReason, MessageForLogPipeline(runningReason), MessageForLogPipeline(pendingReason))

		conditionsSize := len(conditions)

		pendingCond := conditions[conditionsSize-2]
		require.Equal(t, TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionFalse, pendingCond.Status)
		require.Equal(t, pendingReason, pendingCond.Reason)
		pendingCondMsg := PendingTypeDeprecationMsg + MessageForLogPipeline(pendingReason)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)

		runningCond := conditions[conditionsSize-1]
		require.Equal(t, TypeRunning, runningCond.Type)
		require.Equal(t, metav1.ConditionTrue, runningCond.Status)
		require.Equal(t, runningReason, runningCond.Reason)
		runningCondMsg := RunningTypeDeprecationMsg + MessageForLogPipeline(runningReason)
		require.Equal(t, runningCondMsg, runningCond.Message)
		require.Equal(t, generation, runningCond.ObservedGeneration)
		require.NotEmpty(t, runningCond.LastTransitionTime)
	})
}
