package conditions

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func HandlePendingCondition(conditions *[]metav1.Condition, generation int64, reason, origMessage string) {
	pending := metav1.Condition{
		Type:               TypePending,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		ObservedGeneration: generation,
		Message:            PendingTypeDeprecationMsg + origMessage,
	}

	if meta.FindStatusCondition(*conditions, TypeRunning) != nil {
		meta.RemoveStatusCondition(conditions, TypeRunning)
	}

	meta.SetStatusCondition(conditions, pending)
}

func HandleRunningCondition(conditions *[]metav1.Condition, generation int64, runningReason, pendingReason, origRunningMessage, origPendingMessage string) {
	// Set Pending condition to False
	pending := metav1.Condition{
		Type:               TypePending,
		Status:             metav1.ConditionFalse,
		Reason:             pendingReason,
		ObservedGeneration: generation,
		Message:            PendingTypeDeprecationMsg + origPendingMessage,
	}

	meta.SetStatusCondition(conditions, pending)

	// Set Running condition to True
	running := metav1.Condition{
		Type:               TypeRunning,
		Status:             metav1.ConditionTrue,
		Reason:             runningReason,
		ObservedGeneration: generation,
		Message:            RunningTypeDeprecationMsg + origRunningMessage,
	}

	meta.SetStatusCondition(conditions, running)
}
