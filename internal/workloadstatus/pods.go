package workloadstatus

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrOOMKilled          = errors.New("container is OOMKilled")
	ErrContainerCrashLoop = errors.New("container is in crash loop")
	ErrNoPodsDeployed     = errors.New("no pods deployed")
)

const (
	timeThreshold = 5 * time.Minute
)

type ProcessInContainerExitedError struct {
	ExitCode int32
}

func (picee *ProcessInContainerExitedError) Error() string {
	return fmt.Sprintf("container process has exited with status: %d", picee.ExitCode)
}

func isProcessInContainerExitedError(err error) bool {
	var picee *ProcessInContainerExitedError
	return errors.As(err, &picee)
}

type ContainerNotRunningError struct {
	Message string
}

func (cnre *ContainerNotRunningError) Error() string {
	return fmt.Sprintf("container is not running: %s", cnre.Message)
}

func isContainerNotRunningError(err error) bool {
	var cnre *ContainerNotRunningError
	return errors.As(err, &cnre)
}

type PodIsPendingError struct {
	Message string
}

func (pipe *PodIsPendingError) Error() string {
	return fmt.Sprintf("pod is in pending state: %s", pipe.Message)
}

func isPodIsPendingError(err error) bool {
	var pipe *PodIsPendingError
	return errors.As(err, &pipe)
}

type PodIsEvictedError struct {
	Message string
}

func (pie *PodIsEvictedError) Error() string {
	return fmt.Sprintf("pod has been evicted: %s", pie.Message)
}

func isPodIsEvictedError(err error) bool {
	var pie *PodIsEvictedError
	return errors.As(err, &pie)
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
			return &PodIsPendingError{Message: c.Message}
		}
	}
	return nil
}

func checkEviction(status corev1.PodStatus) error {
	if status.Reason == "Evicted" {
		return &PodIsEvictedError{Message: status.Message}
	}
	return nil
}

func checkWaitingPods(c corev1.ContainerStatus) error {
	if c.State.Waiting == nil {
		return nil
	}

	// handle the cases when image is not pulled.
	if c.LastTerminationState.Terminated == nil {
		return fetchWaitingReason(*c.State.Waiting, -1)
	}

	lastTerminatedState := c.LastTerminationState.Terminated
	if lastTerminatedState.Reason == "OOMKilled" && exceededTimeThreshold(lastTerminatedState.StartedAt) {
		return ErrOOMKilled
	}

	if lastTerminatedState.Reason == "Error" && exceededTimeThreshold(lastTerminatedState.StartedAt) {
		return fetchWaitingReason(*c.State.Waiting, lastTerminatedState.ExitCode)
	}
	return nil
}

func fetchWaitingReason(state corev1.ContainerStateWaiting, exitCode int32) error {
	if state.Reason == "CrashLoopBackOff" {
		return ErrContainerCrashLoop
	}
	if exitCode == -1 {
		return &ContainerNotRunningError{Message: state.Reason}
	}
	return &ProcessInContainerExitedError{ExitCode: exitCode}
}

func exceededTimeThreshold(startedAt metav1.Time) bool {
	return time.Since(startedAt.Time) > timeThreshold
}
