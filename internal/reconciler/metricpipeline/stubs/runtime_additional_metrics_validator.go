package stubs

import (
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

type RuntimeAdditionalMetricsValidator struct {
	err error
}

func NewRuntimeAdditionalMetricsValidator(err error) *RuntimeAdditionalMetricsValidator {
	return &RuntimeAdditionalMetricsValidator{
		err: err,
	}
}

func (v *RuntimeAdditionalMetricsValidator) Validate(pipeline *telemetryv1beta1.MetricPipeline) error {
	return v.err
}
