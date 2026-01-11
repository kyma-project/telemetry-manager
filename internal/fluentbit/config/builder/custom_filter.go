package builder

import (
	"fmt"
	"strings"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config"
)

const (
	multilineFilter    = "multiline"
	nonMultilineFilter = "non-multiline"
)

func createCustomFilters(pipeline *telemetryv1beta1.LogPipeline, filterType string) string {
	var filters []string

	for _, filter := range pipeline.Spec.FluentBitFilters {
		customFilterParams := parseMultiline(filter.Custom)
		isMultiline := isMultilineFilter(customFilterParams)

		if (filterType == multilineFilter && !isMultiline) || (filterType == nonMultilineFilter && isMultiline) {
			continue
		}

		builder := NewFilterSectionBuilder()
		for _, p := range customFilterParams {
			builder.AddConfigParam(p.Key, p.Value)
		}

		builder.AddConfigParam("match", fmt.Sprintf("%s.*", pipeline.Name))
		filters = append(filters, builder.Build())
	}

	return strings.Join(filters, "\n")
}

func isMultilineFilter(filter config.ParameterList) bool {
	return filter.ContainsKey("name") && strings.Compare(filter.GetByKey("name").Value, "multiline") == 0
}
