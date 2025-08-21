package stubs

import (
	"context"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type OTTLLogValidator struct {
	err error
}

func NewOTTLLogValidator(err error) *OTTLLogValidator {
	return &OTTLLogValidator{
		err: err,
	}
}

func (o *OTTLLogValidator) Validate(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
	return o.err
}
