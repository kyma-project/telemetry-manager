package conditions

import (
	"errors"
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

type ErrorToMessageConverter struct {
}

// Convert converts an error to a user-friendly message. The message can be
// enhanced by adding additional context which would be useful for the user.
func (etc *ErrorToMessageConverter) Convert(err error) string {
	var pns *workloadstatus.PodIsNotScheduledError
	if errors.As(err, &pns) {
		return fmt.Sprintf(podIsNotScheduled, pns.Message)
	}

	var pipe *workloadstatus.PodIsPendingError
	if errors.As(err, &pipe) {
		if pipe.Reason == "" {
			return fmt.Sprintf(podIsPending, pipe.ContainerName, pipe.Message)
		}
		return fmt.Sprintf(podIsPending, pipe.ContainerName, pipe.Reason)
	}

	var pfe *workloadstatus.PodIsFailingError
	if errors.As(err, &pfe) {
		return fmt.Sprintf(podIsFailed, pfe.Message)
	}

	var ripe *workloadstatus.RolloutInProgressError
	if errors.As(err, &ripe) {
		return podRolloutInProgress
	}

	return ConvertErrToMsg(err)
}
