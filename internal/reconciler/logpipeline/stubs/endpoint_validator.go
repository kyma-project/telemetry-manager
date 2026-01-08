package stubs

import (
	"context"

	"github.com/kyma-project/telemetry-manager/internal/validators/endpoint"
)

type EndpointValidator struct {
	err error
}

func NewEndpointValidator(err error) *EndpointValidator {
	return &EndpointValidator{
		err: err,
	}
}

func (e *EndpointValidator) Validate(ctx context.Context, params endpoint.EndpointValidationParams) error {
	return e.err
}
