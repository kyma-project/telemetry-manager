package telemetry

import (
	"errors"
)

var (
	errIncorrectSecretObject = errors.New("incorrect secret object")
	errIncorrectCRDObject    = errors.New("incorrect CRD object")
)
