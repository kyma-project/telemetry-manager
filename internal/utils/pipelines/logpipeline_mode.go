package pipelines

import (
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type LogPipelineMode int

const (
	OTel LogPipelineMode = iota
	FluentBit
)

func DetermineLogPipelineMode(lp *telemetryv1alpha1.LogPipeline) LogPipelineMode {
	if lp.Spec.Output.OTLP != nil {
		return OTel
	}

	return FluentBit
}
