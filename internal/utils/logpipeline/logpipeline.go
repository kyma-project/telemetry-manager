package logpipeline

import (
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/featureflags"
	sharedtypesutils "github.com/kyma-project/telemetry-manager/internal/utils/sharedtypes"
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

func IsInputValid(i *telemetryv1alpha1.LogPipelineInput) bool {
	return i != nil
}

func IsCustomDefined(o *telemetryv1alpha1.LogPipelineOutput) bool {
	return o.Custom != ""
}

func IsHTTPDefined(o *telemetryv1alpha1.LogPipelineOutput) bool {
	return o.HTTP != nil && sharedtypesutils.IsValid(&o.HTTP.Host)
}

func IsOTLPDefined(o *telemetryv1alpha1.LogPipelineOutput) bool {
	return o.OTLP != nil
}

func IsAnyDefined(o *telemetryv1alpha1.LogPipelineOutput) bool {
	return pluginCount(o) > 0
}

func IsSingleDefined(o *telemetryv1alpha1.LogPipelineOutput) bool {
	return pluginCount(o) == 1
}

func pluginCount(o *telemetryv1alpha1.LogPipelineOutput) int {
	plugins := 0
	if IsCustomDefined(o) {
		plugins++
	}

	if IsHTTPDefined(o) {
		plugins++
	}

	if featureflags.IsEnabled(featureflags.LogPipelineOTLP) && IsOTLPDefined(o) {
		plugins++
	}

	return plugins
}

// ContainsCustomPlugin returns true if the pipeline contains any custom filters or outputs
func ContainsCustomPlugin(lp *telemetryv1alpha1.LogPipeline) bool {
	for _, filter := range lp.Spec.Filters {
		if filter.Custom != "" {
			return true
		}
	}

	return IsCustomDefined(&lp.Spec.Output)
}
