package otel

import (
	"context"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type Validator struct {
	// TODO: Add validator interfaces
}

func (v *Validator) validate(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
	// TODO: Implement validation logic
	return nil
}