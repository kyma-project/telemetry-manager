package stubs

import (
	"context"

	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

type TLSCertValidator struct {
	err error
}

func NewTLSCertValidator(err error) *TLSCertValidator {
	return &TLSCertValidator{
		err: err,
	}
}

func (t *TLSCertValidator) Validate(ctx context.Context, config tlscert.TLSValidationParams) error {
	return t.err
}
