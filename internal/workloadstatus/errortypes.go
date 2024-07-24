package workloadstatus

import (
	"errors"
	"fmt"
)

var (
	//ErrOOMKilled          = errors.New("container is OOMKilled")
	//ErrContainerCrashLoop = errors.New("container is in crash loop")
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

type ImageNotPulledError struct {
	ContainerName string
	Message       string
}

func (inpe *ImageNotPulledError) Error() string {
	return fmt.Sprintf("Image in container: %s cannot be pulled: %s", inpe.ContainerName, inpe.Message)
}

func IsImageNotPulledError(err error) bool {
	var inpe *ImageNotPulledError
	return errors.As(err, &inpe)
}
