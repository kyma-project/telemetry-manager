package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLogPipelineOutput(t *testing.T) {
	tests := []struct {
		name           string
		given          Output
		expectedCustom bool
		expectedHTTP   bool
		expectedLoki   bool
		expectedAny    bool
		expectedSingle bool
	}{
		{
			name:           "custom",
			given:          Output{Custom: "name: null"},
			expectedCustom: true,
			expectedAny:    true,
			expectedSingle: true,
		},
		{
			name:           "http",
			given:          Output{HTTP: &HTTPOutput{Host: ValueType{Value: "localhost"}}},
			expectedHTTP:   true,
			expectedAny:    true,
			expectedSingle: true,
		},
		{
			name:           "loki",
			given:          Output{Loki: &LokiOutput{URL: ValueType{Value: "localhost"}}},
			expectedLoki:   true,
			expectedAny:    true,
			expectedSingle: true,
		},
		{
			name:           "invalid: none defined",
			given:          Output{},
			expectedAny:    false,
			expectedSingle: false,
		},
		{
			name:           "invalid: multiple defined",
			given:          Output{Custom: "name: null", Loki: &LokiOutput{URL: ValueType{Value: "localhost"}}},
			expectedCustom: true,
			expectedLoki:   true,
			expectedAny:    true,
			expectedSingle: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expectedCustom, test.given.IsCustomDefined())
			require.Equal(t, test.expectedHTTP, test.given.IsHTTPDefined())
			require.Equal(t, test.expectedLoki, test.given.IsLokiDefined())
			require.Equal(t, test.expectedAny, test.given.IsAnyDefined())
		})
	}
}

func TestLogPipelineContainsCustomPluginWithCustomFilter(t *testing.T) {
	logPipeline := &LogPipeline{
		Spec: LogPipelineSpec{
			Filters: []Filter{
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
			Output: Output{
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
