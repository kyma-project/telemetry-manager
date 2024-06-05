package stubs

import (
	"context"

	"github.com/kyma-project/telemetry-manager/internal/tlscert"
)

type TLSCertValidator struct {
	err error
}

func NewTLSCertValidator(err error) *TLSCertValidator {
	return &TLSCertValidator{
		err: err,
	}
}

func (t *TLSCertValidator) Validate(ctx context.Context, config tlscert.TLSBundle) error {
	return t.err
}
