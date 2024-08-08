package stubs

import (
	"context"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type EndpointValidator struct {
	err error
}

func NewEndpointValidator(err error) *EndpointValidator {
	return &EndpointValidator{
		err: err,
	}
}

func (e *EndpointValidator) Validate(ctx context.Context, endpoint *telemetryv1alpha1.ValueType) error {
	return e.err
}
