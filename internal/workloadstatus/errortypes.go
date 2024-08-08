package workloadstatus

import (
	"errors"
	"fmt"
)

var (
	ErrNoPodsDeployed    = errors.New("no Pods deployed")
	ErrDaemonSetNotFound = errors.New("DaemonSet is not yet created")
	ErrDaemonSetFetching = errors.New("failed to get DaemonSet")

	ErrDeploymentNotFound1         = errors.New("Deployment is not yet created") //nolint: stylecheck // Deployment is a proper noun
	ErrDeploymentFetching          = errors.New("failed to get Deployment")
	ErrFailedToGetLatestReplicaSet = errors.New("failed to get latest ReplicaSets")
)

type PodIsNotScheduledError struct {
	Message string
}

func (pns *PodIsNotScheduledError) Error() string {
	return fmt.Sprintf("Pod is not scheduled: %s", pns.Message)
}

func IsPodIsNotScheduledError(err error) bool {
	var pns *PodIsNotScheduledError
	return errors.As(err, &pns)
}

type PodIsPendingError struct {
	ContainerName string
	Reason        string
	Message       string
}

func (p PodIsPendingError) Error() string {
	return fmt.Sprintf("Pod is in the pending state: reason: %s, message: %s", p.Reason, p.Message)
}

func IsPodIsPendingError(err error) bool {
	var pipe *PodIsPendingError
	return errors.As(err, &pipe)
}

type PodIsFailingError struct {
	Message string
}

func (pfe *PodIsFailingError) Error() string {
	return fmt.Sprintf("Pod has failed: %s", pfe.Message)
}

func IsPodFailedError(err error) bool {
	var pfe *PodIsFailingError
	return errors.As(err, &pfe)
}

type FailedToListReplicaSetError struct {
	ErrorObj error
}

func (ftlr *FailedToListReplicaSetError) Error() string {
	return fmt.Sprintf("failed to list ReplicaSets: %s", ftlr.ErrorObj.Error())
}

func IsFailedToListReplicaSetErr(err error) bool {
	var ftlr *FailedToListReplicaSetError
	return errors.As(err, &ftlr)
}

type FailedToFetchReplicaSetError struct {
	ErroObj error
}

func (ftfr *FailedToFetchReplicaSetError) Error() string {
	return fmt.Sprintf("failed to fetch ReplicaSets: %s", ftfr.ErroObj.Error())
}

func IsFailedToFetchReplicaSetError(err error) bool {
	var ftfr *FailedToFetchReplicaSetError
	return errors.As(err, &ftfr)
}

type RolloutInProgressError struct {
}

func (ripe *RolloutInProgressError) Error() string {
	return "Rollout is in progress"
}

func IsRolloutInProgressError(err error) bool {
	var ripe *RolloutInProgressError
	return errors.As(err, &ripe)
}
