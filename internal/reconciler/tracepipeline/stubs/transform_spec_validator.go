package stubs

import (
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type TransformSpecValidator struct {
	err error
}

func NewTransformSpecValidator(err error) *TransformSpecValidator {
	return &TransformSpecValidator{
		err: err,
	}
}

func (o *TransformSpecValidator) Validate(transforms []telemetryv1alpha1.TransformSpec) error {
	return o.err
}
