package builder

import (
	"testing"

	"github.com/stretchr/testify/require"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func TestCreateCustomFilters(t *testing.T) {
	tests := []struct {
		name       string
		pipeline   *telemetryv1alpha1.LogPipeline
		filterType string
		want       string
	}{
		{
			name: "Test Multiline Filter",
			pipeline: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Filters: []telemetryv1alpha1.Filter{
						{
							Custom: `
								name multiline
								`,
						},
					},
				},
			},
			filterType: multilineFilter,
			want:       "[FILTER]\n    name  multiline\n    match .*\n\n",
		},
		{
			name: "Test Non-Multiline Filter",
			pipeline: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Filters: []telemetryv1alpha1.Filter{
						{
							Custom: `
								name grep
								`,
						},
					},
				},
			},
			filterType: nonMultilineFilter,
			want:       "[FILTER]\n    name  grep\n    match .*\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filterConf := createCustomFilters(tt.pipeline, tt.filterType)
			require.Equal(t, filterConf, tt.want)
		})
	}
}
