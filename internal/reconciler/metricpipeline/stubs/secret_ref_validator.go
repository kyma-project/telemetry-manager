package stubs

import (
	"context"

	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
)

type SecretRefValidator struct {
	err error
}

func NewSecretRefValidator(err error) *SecretRefValidator {
	return &SecretRefValidator{
		err: err,
	}
}

func (s *SecretRefValidator) Validate(ctx context.Context, getter secretref.Getter) error {
	return s.err
}
