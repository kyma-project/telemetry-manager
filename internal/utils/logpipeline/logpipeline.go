package logpipeline

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
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

// ContainsCustomPlugin returns true if the pipeline contains any custom filters or outputs
func ContainsCustomPlugin(lp *telemetryv1alpha1.LogPipeline) bool {
	for _, filter := range lp.Spec.FluentBitFilters {
		if filter.Custom != "" {
			return true
		}
	}

	return IsCustomDefined(&lp.Spec.Output)
}

func GetPipelinesForType(ctx context.Context, client client.Client, mode Mode) ([]telemetryv1alpha1.LogPipeline, error) {
	var allPipelines telemetryv1alpha1.LogPipelineList
	if err := client.List(ctx, &allPipelines); err != nil {
		return nil, fmt.Errorf("failed to get all log pipelines while syncing Fluent Bit ConfigMaps: %w", err)
	}

	var filteredList []telemetryv1alpha1.LogPipeline

	for _, lp := range allPipelines.Items {
		if GetOutputType(&lp) == mode {
			filteredList = append(filteredList, lp)
		}
	}

	return filteredList, nil
}

func GetOutputType(t *telemetryv1alpha1.LogPipeline) Mode {
	if t.Spec.Output.OTLP != nil {
		return OTel
	}

	return FluentBit
}

func IsOTLPInputEnabled(input telemetryv1alpha1.LogPipelineInput) bool {
	return input.OTLP == nil || !input.OTLP.Disabled
}
