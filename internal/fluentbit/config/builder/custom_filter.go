package builder

import (
	"fmt"
	"strings"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func createCustomFilters(pipeline *telemetryv1alpha1.LogPipeline) string {
	var filters []string

	for _, filter := range pipeline.Spec.Filters {
		customFilterParams := parseMultiline(filter.Custom)
		// skip if the filter is a multiline filter, multiline filter should be first filter in the pipeline chain
		// see for more details https://docs.fluentbit.io/manual/pipeline/filters/multiline-stacktrace
		if customFilterParams.ContainsKey("name") && strings.Compare(customFilterParams.GetByKey("name").Value, "multiline") == 0 {
			continue
		}
		builder := NewFilterSectionBuilder()
		for _, p := range customFilterParams {
			builder.AddConfigParam(p.Key, p.Value)
		}
		builder.AddConfigParam("match", fmt.Sprintf("%s.*", pipeline.Name))
		filters = append(filters, builder.Build())
	}

	return strings.Join(filters, "")
}

func createCustomMultilineFilters(pipeline *telemetryv1alpha1.LogPipeline) string {
	var filters []string

	for _, filter := range pipeline.Spec.Filters {
		customFilterParams := parseMultiline(filter.Custom)
		if customFilterParams.ContainsKey("name") && strings.Compare(customFilterParams.GetByKey("name").Value, "multiline") == 0 {
			builder := NewFilterSectionBuilder()
			for _, p := range customFilterParams {
				builder.AddConfigParam(p.Key, p.Value)
			}
			builder.AddConfigParam("match", fmt.Sprintf("%s.*", pipeline.Name))
			filters = append(filters, builder.Build())
		}
	}

	return strings.Join(filters, "")
}
