package conditions

import (
	"errors"
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

const (
	//containerNotRunning = "Container: %s in not in running state due to: %s"
	podIsNotScheduled    = "Pod not scheduled: %s"
	podIsPending         = "Pod is in pending state as container: %s is not running due to: %s"
	podIsFailed          = "Pod is in failed state due to: %s"
	podRolloutInProgress = "Pods are being started/updated"
)

type ErrorToMessageConverter struct {
}

func (etc *ErrorToMessageConverter) Convert(err error) string {
	if errors.Is(err, &workloadstatus.PodIsNotScheduledError{}) {
		podNotScheduled := err.(*workloadstatus.PodIsNotScheduledError)
		return fmt.Sprintf(podIsNotScheduled, podNotScheduled.Message)
	}

	if workloadstatus.IsPodIsPendingError(err) {
		podPending := err.(*workloadstatus.PodIsPendingError)
		if podPending.Reason == "" {
			return fmt.Sprintf(podIsPending, podPending.ContainerName, podPending.Message)
		}
		return fmt.Sprintf(podIsPending, podPending.ContainerName, podPending.Reason)
	}

	if workloadstatus.IsPodFailedError(err) {
		podFailed := err.(*workloadstatus.PodIsFailingError)
		return fmt.Sprintf(podIsFailed, podFailed.Message)
	}

	if workloadstatus.IsRolloutInProgressError(err) {
		return podRolloutInProgress
	}

	return ConvertErrToMsg(err)
}
