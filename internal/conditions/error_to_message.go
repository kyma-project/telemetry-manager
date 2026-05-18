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
	if pns, ok := errors.AsType[*workloadstatus.PodIsNotScheduledError](err); ok {
		return fmt.Sprintf(podIsNotScheduled, pns.Message)
	}

	if pipe, ok := errors.AsType[*workloadstatus.PodIsPendingError](err); ok {
		if pipe.Reason == "" {
			return fmt.Sprintf(podIsPending, pipe.ContainerName, pipe.Message, pipe.ContainerName)
		}

		return fmt.Sprintf(podIsPending, pipe.ContainerName, pipe.Reason, pipe.ContainerName)
	}

	if pfe, ok := errors.AsType[*workloadstatus.PodIsFailingError](err); ok {
		return fmt.Sprintf(podIsFailed, pfe.Message)
	}

	if ripe, _ := errors.AsType[*workloadstatus.RolloutInProgressError](err); ripe != nil {
		return podRolloutInProgress
	}

	if ftlr, ok := errors.AsType[*workloadstatus.FailedToListReplicaSetError](err); ok {
		return ConvertErrToMsg(ftlr.ErrorObj)
	}

	if ftfr, ok := errors.AsType[*workloadstatus.FailedToFetchReplicaSetError](err); ok {
		return ConvertErrToMsg(ftfr.ErroObj)
	}

	return ConvertErrToMsg(err)
}
