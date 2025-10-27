package stubs

import (
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type FilterSpecValidator struct {
	err error
}

func NewFilterSpecValidator(err error) *FilterSpecValidator {
	return &FilterSpecValidator{
		err: err,
	}
}

func (o *FilterSpecValidator) Validate(transforms []telemetryv1alpha1.FilterSpec) error {
	return o.err
}
