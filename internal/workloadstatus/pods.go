package workloadstatus

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Status from a pod consists of PodConditions and ContainerStatuses
// PodPhase tells the current state of Pod which can be Pending, Running, Succeeded, Failed, Unknown
// The table below shows the various scenarios anc compares with
// podPhase, PodScheduled (under podConditions), ContainerStatus.State, ContainerStatus.LastState
// +----------------+-----------------+-----------------+-----------------+-----------------+-----------------+
// | Scenario       | PodPhase | Pod Scheduled | ContainerStatus.State           | ContainerStatus.LastState |
// |  Crashbackloop | Running  | True          | State.Waiting.Reason: CrashLoopBackOff| exitCode: 1, Reason: Error|
// |  OOMKilled     | Running  | True          | State.Waiting.Reason: OOMKilled    | exitCode: 137, Reason: OOMKilled|
// | PVC not found  | Pending  | False, Reason: Unschedulable        | - | -                       |
// | ImagePullBackOff| Pending | True        | State.Waiting.Reason: ErrImagePul|-|
// | Evicted        | Failed   | True             | - | -                       | only Status.Message is set
func checkPodStatus(ctx context.Context, c client.Client, namespace string, selector *metav1.LabelSelector) error {
	var pods corev1.PodList

	if err := c.List(ctx, &pods, client.InNamespace(namespace), client.MatchingLabels(selector.MatchLabels)); err != nil {
		return err
	}

	if len(pods.Items) == 0 {
		return ErrNoPodsDeployed
	}

	for _, pod := range pods.Items {
		//check if Pod is in running state & all containers are ready.
		podReadyCondition := findPodConditions(pod.Status.Conditions, corev1.PodReady)
		if pod.Status.Phase == corev1.PodRunning && podReadyCondition.Status == corev1.ConditionTrue {
			continue
		}
		// Check if Pods are in Pending state
		if err := checkPodPendingState(pod.Status); err != nil {
			return err
		}
		// check pods are in failed State
		if err := checkPodFailedState(pod.Status); err != nil {
			return err
		}
		// Check is Pod is running state and if there is some issue with one of the containers
		for _, c := range pod.Status.ContainerStatuses {
			// Check if pod is terminated
			if err := checkPodsWaitingState(pod.Status, c); err != nil {
				return err
			}
		}
	}

	return nil
}

func checkPodPendingState(status corev1.PodStatus) error {
	if status.Phase != corev1.PodPending {
		return nil
	}

	condition := findPodConditions(status.Conditions, corev1.PodScheduled)
	if condition.Status == corev1.ConditionFalse {
		return &PodIsPendingError{Message: condition.Message}
	}

	for _, c := range status.ContainerStatuses {
		if c.State.Waiting != nil {
			// During the restart of the pod can be stuck in PodIntializing and ContainerCreating state for
			// long which is not an error state, so we skip this state
			if c.State.Waiting.Message != "PodInitializing" && c.State.Waiting.Message != "ContainerCreating" {
				if c.State.Waiting.Reason != "" {
					return &PodIsPendingError{Message: c.State.Waiting.Reason}
				}
				return &PodIsPendingError{Message: c.State.Waiting.Message}
			}

		}
	}

	// We skip checking the state of each container here as they are not ready and hence the state would be false.
	// Returning waiting reason would be wrong as the pod might be still starting up.
	return nil
}

func checkPodFailedState(status corev1.PodStatus) error {
	if status.Phase != corev1.PodFailed {
		return nil
	}

	return &PodIsFailedError{Message: status.Message}
}

func checkPodsWaitingState(status corev1.PodStatus, c corev1.ContainerStatus) error {
	if status.Phase != corev1.PodRunning || c.State.Waiting == nil {
		return nil
	}

	if c.LastTerminationState.Terminated != nil {
		lastTerminatedState := c.LastTerminationState.Terminated

		if lastTerminatedState.Reason != "" {
			return &ContainerNotRunningError{Message: lastTerminatedState.Reason}
		}
	}

	// handle rest of error states when lastTerminatedState is not set
	return &ContainerNotRunningError{Message: c.State.Waiting.Message}
}

func findPodConditions(conditions []corev1.PodCondition, s corev1.PodConditionType) corev1.PodCondition {
	for _, c := range conditions {
		if c.Type == s {
			return c
		}
	}
	return corev1.PodCondition{}

}
