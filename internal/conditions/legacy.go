package conditions

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func HandlePendingCondition(ctx context.Context, conditions *[]metav1.Condition, generation int64, reason, resourceName string, messageMap map[string]string) {
	log := logf.FromContext(ctx)

	pending := metav1.Condition{
		Type:               TypePending,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		ObservedGeneration: generation,
		Message:            MessageFor(reason, messageMap),
	}
	pending.Message = PendingTypeDeprecationMsg + pending.Message

	if meta.FindStatusCondition(*conditions, TypeRunning) != nil {
		log.V(1).Info(fmt.Sprintf("Updating the status of %s: Removing the Running condition", resourceName))
		meta.RemoveStatusCondition(conditions, TypeRunning)
	}

	log.V(1).Info(fmt.Sprintf("Updating the status of %s: Setting the Pending condition to True", resourceName))
	meta.SetStatusCondition(conditions, pending)
}

func HandleRunningCondition(ctx context.Context, conditions *[]metav1.Condition, generation int64, runningReason, pendingReason, resourceName string, messageMap map[string]string) {
	log := logf.FromContext(ctx)

	// Set Pending condition to False
	pending := metav1.Condition{
		Type:               TypePending,
		Status:             metav1.ConditionFalse,
		Reason:             pendingReason,
		ObservedGeneration: generation,
		Message:            MessageFor(pendingReason, messageMap),
	}

	pending.Message = PendingTypeDeprecationMsg + pending.Message
	log.V(1).Info(fmt.Sprintf("Updating the status of %s: Setting the Pending condition to False", resourceName))
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
	log.V(1).Info(fmt.Sprintf("Updating the status of %s: Setting the Running condition to True", resourceName))
	meta.SetStatusCondition(conditions, running)
}
