package stubs

import (
	"context"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type SecretRefValidator struct {
	err error
}

func NewSecretRefValidator(err error) *SecretRefValidator {
	return &SecretRefValidator{
		err: err,
	}
}

func (s *SecretRefValidator) ValidateLogPipeline(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
	return s.err
}
