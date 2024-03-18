package conditions

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func HandlePendingCondition(conditions *[]metav1.Condition, generation int64, reason string, messageMap map[string]string) {
	pending := metav1.Condition{
		Type:               TypePending,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		ObservedGeneration: generation,
		Message:            MessageFor(reason, messageMap),
	}
	pending.Message = PendingTypeDeprecationMsg + pending.Message

	if meta.FindStatusCondition(*conditions, TypeRunning) != nil {
		meta.RemoveStatusCondition(conditions, TypeRunning)
	}

	meta.SetStatusCondition(conditions, pending)
}

func HandleRunningCondition(conditions *[]metav1.Condition, generation int64, runningReason, pendingReason string, messageMap map[string]string) {
	// Set Pending condition to False
	pending := metav1.Condition{
		Type:               TypePending,
		Status:             metav1.ConditionFalse,
		Reason:             pendingReason,
		ObservedGeneration: generation,
		Message:            MessageFor(pendingReason, messageMap),
	}

	pending.Message = PendingTypeDeprecationMsg + pending.Message
	meta.SetStatusCondition(conditions, pending)

	// Set Running condition to True
	running := metav1.Condition{
		Type:               TypeRunning,
		Status:             metav1.ConditionTrue,
		Reason:             runningReason,
		ObservedGeneration: generation,
		Message:            MessageFor(runningReason, messageMap),
	}

	running.Message = RunningTypeDeprecationMsg + running.Message
	meta.SetStatusCondition(conditions, running)
}
