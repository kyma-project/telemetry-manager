package stubs

import (
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

type FilterSpecValidator struct {
	err error
}

func NewFilterSpecValidator(err error) *FilterSpecValidator {
	return &FilterSpecValidator{
		err: err,
	}
}

func (o *FilterSpecValidator) Validate(transforms []telemetryv1beta1.FilterSpec) error {
	return o.err
}
