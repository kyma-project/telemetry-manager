package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLogPipelineOutput(t *testing.T) {
	tests := []struct {
		name           string
		given          LogPipelineOutput
		expectedCustom bool
		expectedHTTP   bool
		expectedLoki   bool
		expectedAny    bool
		expectedSingle bool
	}{
		{
			name:           "custom",
			given:          LogPipelineOutput{Custom: "name: null"},
			expectedCustom: true,
			expectedAny:    true,
			expectedSingle: true,
		},
		{
			name:           "http",
			given:          LogPipelineOutput{HTTP: &LogPipelineHTTPOutput{Host: ValueType{Value: "localhost"}}},
			expectedHTTP:   true,
			expectedAny:    true,
			expectedSingle: true,
		},
		{
			name:           "invalid: none defined",
			given:          LogPipelineOutput{},
			expectedAny:    false,
			expectedSingle: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expectedCustom, test.given.IsCustomDefined())
			require.Equal(t, test.expectedHTTP, test.given.IsHTTPDefined())
			require.Equal(t, test.expectedAny, test.given.IsAnyDefined())
		})
	}
}

func TestLogPipelineContainsCustomPluginWithCustomFilter(t *testing.T) {
	logPipeline := &LogPipeline{
		Spec: LogPipelineSpec{
			Filters: []LogPipelineFilter{
				{Custom: `
    Name    some-filter`,
				},
			},
		},
	}

	result := logPipeline.ContainsCustomPlugin()
	require.True(t, result)
}

func TestLogPipelineContainsCustomPluginWithCustomOutput(t *testing.T) {
	logPipeline := &LogPipeline{
		Spec: LogPipelineSpec{
			Output: LogPipelineOutput{
				Custom: `
    Name    some-output`,
			},
		},
	}

	result := logPipeline.ContainsCustomPlugin()
	require.True(t, result)
}

func TestLogPipelineContainsCustomPluginWithoutAny(t *testing.T) {
	logPipeline := &LogPipeline{Spec: LogPipelineSpec{}}

	result := logPipeline.ContainsCustomPlugin()
	require.False(t, result)
}
