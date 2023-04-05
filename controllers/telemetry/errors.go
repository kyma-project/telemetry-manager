package telemetry

import (
	"errors"
)

var (
	errIncorrectSecretObject    = errors.New("incorrect secret object")
	errIncorrectDaemonSetObject = errors.New("incorrect daemon set object")
)
