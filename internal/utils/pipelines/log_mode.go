package logpipeline

import (
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type LogMode int

const (
	OTel LogMode = iota
	FluentBit
)

func PipelineMode(lp *telemetryv1alpha1.LogPipeline) LogMode {
	if lp.Spec.Output.OTLP != nil {
		return OTel
	}

	return FluentBit
}
