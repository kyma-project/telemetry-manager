package builder

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

func TestCreateCustomFilters(t *testing.T) {
	testPipeline := &telemetryv1beta1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: telemetryv1beta1.LogPipelineSpec{
			FluentBitFilters: []telemetryv1beta1.FluentBitFilter{
				{
					Custom: `
								name multiline
								`,
				},
				{
					Custom: `
								name grep
								`,
				},
			},
		},
	}

	tests := []struct {
		name       string
		pipeline   *telemetryv1beta1.LogPipeline
		filterType string
		want       string
	}{
		{
			name:       "Test Multiline Filter",
			pipeline:   testPipeline,
			filterType: multilineFilter,
			want:       "[FILTER]\n    name  multiline\n    match foo.*\n\n",
		},
		{
			name:       "Test Non-Multiline Filter",
			pipeline:   testPipeline,
			filterType: nonMultilineFilter,
			want:       "[FILTER]\n    name  grep\n    match foo.*\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filterConf := createCustomFilters(tt.pipeline, tt.filterType)
			require.Equal(t, filterConf, tt.want)
		})
	}
}
