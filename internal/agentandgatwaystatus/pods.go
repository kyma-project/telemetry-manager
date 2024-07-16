package agentandgatwaystatus

import (
	"context"
	"errors"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

var (
	ErrOOMKilled          = errors.New("container is OOMKilled")
	ErrContainerCrashLoop = errors.New("container is in crash loop")
	ErrNoPodsDeployed     = errors.New("no pods deployed")
)

const (
	timeThreshold               = 5 * time.Minute
	ErrProcessInContainerExited = "container process has exited with status: %s"
	ErrPodIsPending             = "pod is in pending state: %s"
	ErrPodEvicted               = "pod has been evicted: %s"
)

var errMap = map[string]string{
	"podIsEvicted":             ErrPodEvicted,
	"podIsPending":             ErrPodIsPending,
	"processInContainerExited": ErrProcessInContainerExited,
}

type PodsError struct {
	errorString string
	message     string
}

func (pe *PodsError) Error() string {
	if err, ok := errMap[pe.errorString]; ok {
		return fmt.Sprintf(err, pe.message)
	}
	if err, ok := errMap[pe.errorString]; ok {
		return fmt.Sprintf(err, pe.message)
	}
	if err, ok := errMap[pe.errorString]; ok {
		return fmt.Sprintf(err, pe.message)
	}
	return fmt.Sprintf("unknown error: %s", pe.errorString)
}

func isPodError(err error) bool {
	var podsError *PodsError
	return errors.As(err, &podsError)
}

func checkPodStatus(ctx context.Context, c client.Client, namespace string, selector *metav1.LabelSelector) error {
	var pods corev1.PodList

	if err := c.List(ctx, &pods, client.InNamespace(namespace), client.MatchingLabels(selector.MatchLabels)); err != nil {
		return err
	}
	if len(pods.Items) == 0 {
		return ErrNoPodsDeployed
	}

	for _, pod := range pods.Items {
		// Check for pending pods
		if err := checkPendingState(pod.Status); err != nil {
			return err
		}
		// check pod status for eviction
		if err := checkEviction(pod.Status); err != nil {
			return err
		}
		for _, c := range pod.Status.ContainerStatuses {
			// Check if pod is terminated
			if err := checkWaitingPods(c); err != nil {
				return err
			}
		}
	}

	return nil
}

func checkPendingState(status corev1.PodStatus) error {
	if status.Phase != corev1.PodPending {
		return nil
	}
	for _, c := range status.Conditions {
		if c.Status == corev1.ConditionFalse && exceededTimeThreshold(c.LastTransitionTime) {
			return &PodsError{errorString: "podIsPending", message: c.Message}
		}
	}
	return nil
}

func checkEviction(status corev1.PodStatus) error {
	if status.Reason == "Evicted" {
		return &PodsError{errorString: "podIsEvicted", message: status.Message}
	}
	return nil
}

func checkWaitingPods(c corev1.ContainerStatus) error {
	if c.State.Waiting == nil {
		return nil
	}
	lastTerminatedState := c.LastTerminationState.Terminated
	if lastTerminatedState.Reason == "OOMKilled" && exceededTimeThreshold(lastTerminatedState.StartedAt) {
		return ErrOOMKilled
	}

	if lastTerminatedState.Reason == "Error" && exceededTimeThreshold(lastTerminatedState.StartedAt) {
		if c.State.Waiting != nil {
			if c.State.Waiting.Reason == "CrashLoopBackOff" {
				return ErrContainerCrashLoop
			}
		}
		return &PodsError{errorString: "processInContainerExited", message: string(lastTerminatedState.ExitCode)}
	}
	return nil
}

func exceededTimeThreshold(startedAt metav1.Time) bool {
	return time.Now().Sub(startedAt.Time) > timeThreshold
}
