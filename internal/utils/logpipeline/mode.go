package logpipeline

import (
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type Mode int

const (
	OTel Mode = iota
	FluentBit
)

func PipelineMode(lp *telemetryv1alpha1.LogPipeline) Mode {
	if lp.Spec.Output.OTLP != nil {
		return OTel
	}

	return FluentBit
}
