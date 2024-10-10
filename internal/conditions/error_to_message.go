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
			return fmt.Sprintf(podIsPending, pipe.ContainerName, pipe.Message, pipe.ContainerName)
		}

		return fmt.Sprintf(podIsPending, pipe.ContainerName, pipe.Reason, pipe.ContainerName)
	}

	var pfe *workloadstatus.PodIsFailingError
	if errors.As(err, &pfe) {
		return fmt.Sprintf(podIsFailed, pfe.Message)
	}

	var ripe *workloadstatus.RolloutInProgressError
	if errors.As(err, &ripe) {
		return podRolloutInProgress
	}

	var ftlr *workloadstatus.FailedToListReplicaSetError
	if errors.As(err, &ftlr) {
		return ConvertErrToMsg(ftlr.ErrorObj)
	}

	var ftfr *workloadstatus.FailedToFetchReplicaSetError
	if errors.As(err, &ftfr) {
		return ConvertErrToMsg(ftfr.ErroObj)
	}

	return ConvertErrToMsg(err)
}
