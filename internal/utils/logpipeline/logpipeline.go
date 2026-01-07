package logpipeline

import (
	"context"
	"fmt"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	sharedtypesutils "github.com/kyma-project/telemetry-manager/internal/utils/sharedtypes"
)

type Mode int

const (
	OTel Mode = iota
	FluentBit
)

func PipelineMode(lp *telemetryv1beta1.LogPipeline) Mode {
	if lp.Spec.Output.OTLP != nil {
		return OTel
	}

	return FluentBit
}

func IsCustomOutputDefined(o *telemetryv1beta1.LogPipelineOutput) bool {
	return o.FluentBitCustom != ""
}

func IsHTTPOutputDefined(o *telemetryv1beta1.LogPipelineOutput) bool {
	return o.FluentBitHTTP != nil && sharedtypesutils.IsValid(&o.FluentBitHTTP.Host)
}

func IsVariablesDefined(v []telemetryv1beta1.FluentBitVariable) bool {
	return len(v) > 0
}

func IsFilesDefined(v []telemetryv1beta1.FluentBitFile) bool {
	return len(v) > 0
}

func IsRuntimeInputEnabled(i *telemetryv1beta1.LogPipelineInput) bool {
	return i.Runtime != nil && ptr.Deref(i.Runtime.Enabled, true)
}

// ContainsCustomPlugin returns true if the pipeline contains any custom filters or outputs
func ContainsCustomPlugin(lp *telemetryv1beta1.LogPipeline) bool {
	return IsCustomOutputDefined(&lp.Spec.Output) || IsCustomFilterDefined(lp.Spec.FluentBitFilters)
}

func IsCustomFilterDefined(filters []telemetryv1beta1.FluentBitFilter) bool {
	for _, filter := range filters {
		if filter.Custom != "" {
			return true
		}
	}

	return false
}

func GetPipelinesForType(ctx context.Context, client client.Client, mode Mode) ([]telemetryv1beta1.LogPipeline, error) {
	var allPipelines telemetryv1beta1.LogPipelineList
	if err := client.List(ctx, &allPipelines); err != nil {
		return nil, fmt.Errorf("failed to get all log pipelines while syncing Fluent Bit ConfigMaps: %w", err)
	}

	var filteredList []telemetryv1beta1.LogPipeline

	for _, lp := range allPipelines.Items {
		if GetOutputType(&lp) == mode {
			filteredList = append(filteredList, lp)
		}
	}

	return filteredList, nil
}

func GetOutputType(t *telemetryv1beta1.LogPipeline) Mode {
	if t.Spec.Output.OTLP != nil {
		return OTel
	}

	return FluentBit
}
