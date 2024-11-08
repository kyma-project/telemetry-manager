package pipelines

import (
	"testing"

	"github.com/stretchr/testify/require"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func TestLogPipelineOutput(t *testing.T) {
	tests := []struct {
		name           string
		given          telemetryv1alpha1.LogPipelineOutput
		expectedCustom bool
		expectedHTTP   bool
		expectedLoki   bool
		expectedAny    bool
		expectedSingle bool
	}{
		{
			name:           "custom",
			given:          telemetryv1alpha1.LogPipelineOutput{Custom: "name: null"},
			expectedCustom: true,
			expectedAny:    true,
			expectedSingle: true,
		},
		{
			name:           "http",
			given:          telemetryv1alpha1.LogPipelineOutput{HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{Host: telemetryv1alpha1.ValueType{Value: "localhost"}}},
			expectedHTTP:   true,
			expectedAny:    true,
			expectedSingle: true,
		},
		{
			name:           "invalid: none defined",
			given:          telemetryv1alpha1.LogPipelineOutput{},
			expectedAny:    false,
			expectedSingle: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expectedCustom, IsCustomDefined(&test.given))
			require.Equal(t, test.expectedHTTP, IsHTTPDefined(&test.given))
			require.Equal(t, test.expectedAny, IsAnyDefined(&test.given))
		})
	}
}

func TestLogPipelineContainsCustomPluginWithCustomFilter(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Filters: []telemetryv1alpha1.LogPipelineFilter{
				{Custom: `
    Name    some-filter`,
				},
			},
		},
	}

	result := ContainsCustomPlugin(logPipeline)
	require.True(t, result)
}

func TestLogPipelineContainsCustomPluginWithCustomOutput(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.LogPipelineOutput{
				Custom: `
    Name    some-output`,
			},
		},
	}

	result := ContainsCustomPlugin(logPipeline)
	require.True(t, result)
}

func TestLogPipelineContainsCustomPluginWithoutAny(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{Spec: telemetryv1alpha1.LogPipelineSpec{}}

	result := ContainsCustomPlugin(logPipeline)
	require.False(t, result)
}
