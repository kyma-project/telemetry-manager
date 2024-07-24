package workloadstatus

import (
	"errors"
	"fmt"
)

var (
	ErrNoPodsDeployed = errors.New("no pods deployed")
)

type ContainerNotRunningError struct {
	Message string
}

func (cnre *ContainerNotRunningError) Error() string {
	return fmt.Sprintf("Container is not running: %s", cnre.Message)
}

func IsContainerNotRunningError(err error) bool {
	var cnre *ContainerNotRunningError
	return errors.As(err, &cnre)
}

type PodIsPendingError struct {
	Message string
}

func (pipe *PodIsPendingError) Error() string {
	return fmt.Sprintf("Pod is in pending state: %s", pipe.Message)
}

func IsPodIsPendingError(err error) bool {
	var pipe *PodIsPendingError
	return errors.As(err, &pipe)
}

type PodIsFailedError struct {
	Message string
}

func (pfe *PodIsFailedError) Error() string {
	return fmt.Sprintf("Pod has failed: %s", pfe.Message)
}

func IsPodFailedError(err error) bool {
	var pfe *PodIsFailedError
	return errors.As(err, &pfe)
}

type FailedToListReplicaSetErr struct {
	Message string
}

func (ftlr *FailedToListReplicaSetErr) Error() string {
	return fmt.Sprintf("failed to list ReplicaSets: %s", ftlr.Message)
}

func IsFailedToListReplicaSetErr(err error) bool {
	var ftlr *FailedToListReplicaSetErr
	return errors.As(err, &ftlr)
}

type FailedToFetchReplicaSetErr struct {
	Message string
}

func (ftfr *FailedToFetchReplicaSetErr) Error() string {
	return fmt.Sprintf("failed to fetch ReplicaSets: %s", ftfr.Message)
}

func IsFailedToFetchReplicaSetErr(err error) bool {
	var ftfr *FailedToFetchReplicaSetErr
	return errors.As(err, &ftfr)
}
