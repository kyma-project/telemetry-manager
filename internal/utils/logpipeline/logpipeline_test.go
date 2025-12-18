package logpipeline

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
			given:          telemetryv1alpha1.LogPipelineOutput{FluentBitCustom: "name: null"},
			expectedCustom: true,
			expectedAny:    true,
			expectedSingle: true,
		},
		{
			name:           "http",
			given:          telemetryv1alpha1.LogPipelineOutput{FluentBitHTTP: &telemetryv1alpha1.FluentBitHTTPOutput{Host: telemetryv1alpha1.ValueType{Value: "localhost"}}},
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
			require.Equal(t, test.expectedCustom, IsCustomOutputDefined(&test.given))
			require.Equal(t, test.expectedHTTP, IsHTTPOutputDefined(&test.given))
			require.Equal(t, test.expectedAny, IsCustomOutputDefined(&test.given) || IsHTTPOutputDefined(&test.given) || test.given.OTLP != nil)
		})
	}
}

func TestLogPipelineContainsCustomPluginWithCustomFilter(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			FluentBitFilters: []telemetryv1alpha1.FluentBitFilter{
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
				FluentBitCustom: `
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
