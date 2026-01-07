package stubs

import (
	"context"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

type SecretRefValidator struct {
	err error
}

func NewSecretRefValidator(err error) *SecretRefValidator {
	return &SecretRefValidator{
		err: err,
	}
}

func (s *SecretRefValidator) ValidateMetricPipeline(ctx context.Context, pipeline *telemetryv1beta1.MetricPipeline) error {
	return s.err
}
