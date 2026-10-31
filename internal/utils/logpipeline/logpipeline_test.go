package logpipeline

import (
	"testing"

	"github.com/stretchr/testify/require"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

func TestLogPipelineOutput(t *testing.T) {
	tests := []struct {
		name           string
		given          telemetryv1beta1.LogPipelineOutput
		expectedCustom bool
		expectedHTTP   bool
		expectedLoki   bool
		expectedAny    bool
		expectedSingle bool
	}{
		{
			name:           "custom",
			given:          telemetryv1beta1.LogPipelineOutput{FluentBitCustom: "name: null"},
			expectedCustom: true,
			expectedAny:    true,
			expectedSingle: true,
		},
		{
			name:           "http",
			given:          telemetryv1beta1.LogPipelineOutput{FluentBitHTTP: &telemetryv1beta1.FluentBitHTTPOutput{Host: telemetryv1beta1.ValueType{Value: "localhost"}}},
			expectedHTTP:   true,
			expectedAny:    true,
			expectedSingle: true,
		},
		{
			name:           "invalid: none defined",
			given:          telemetryv1beta1.LogPipelineOutput{},
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

func TestLogPipelineContainsCustomPlugin(t *testing.T) {
	tests := []struct {
		name     string
		given    telemetryv1beta1.LogPipelineSpec
		expected bool
	}{
		{
			name: "custom filter",
			given: telemetryv1beta1.LogPipelineSpec{
				FluentBitFilters: []telemetryv1beta1.FluentBitFilter{
					{Custom: `
    Name    some-filter`},
				},
			},
			expected: true,
		},
		{
			name: "custom output",
			given: telemetryv1beta1.LogPipelineSpec{
				Output: telemetryv1beta1.LogPipelineOutput{
					FluentBitCustom: `
    Name    some-output`,
				},
			},
			expected: true,
		},
		{
			name:     "no custom plugins",
			given:    telemetryv1beta1.LogPipelineSpec{},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logPipeline := &telemetryv1beta1.LogPipeline{Spec: test.given}
			require.Equal(t, test.expected, ContainsCustomPlugin(logPipeline))
		})
	}
}

func TestCustomOutputPluginName(t *testing.T) {
	tests := []struct {
		name     string
		given    telemetryv1beta1.LogPipelineOutput
		expected string
	}{
		{
			name:     "name present",
			given:    telemetryv1beta1.LogPipelineOutput{FluentBitCustom: "    Name    opensearch\n    Host    localhost"},
			expected: "opensearch",
		},
		{
			name:     "name present uppercase key",
			given:    telemetryv1beta1.LogPipelineOutput{FluentBitCustom: "    NAME    Loki\n    Host    localhost"},
			expected: "loki",
		},
		{
			name:     "name absent",
			given:    telemetryv1beta1.LogPipelineOutput{FluentBitCustom: "    Host    localhost"},
			expected: "unknown",
		},
		{
			name:     "empty config",
			given:    telemetryv1beta1.LogPipelineOutput{},
			expected: "unknown",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, CustomOutputPluginName(&test.given))
		})
	}
}

func TestCustomFilterPluginNames(t *testing.T) {
	tests := []struct {
		name     string
		given    []telemetryv1beta1.FluentBitFilter
		expected []string
	}{
		{
			name: "single custom filter",
			given: []telemetryv1beta1.FluentBitFilter{
				{Custom: "    Name    grep\n    Regex   log foo"},
			},
			expected: []string{"grep"},
		},
		{
			name: "multiple custom filters",
			given: []telemetryv1beta1.FluentBitFilter{
				{Custom: "    Name    grep\n    Regex   log foo"},
				{Custom: "    Name    Modify\n    Add     key val"},
			},
			expected: []string{"grep", "modify"},
		},
		{
			name:     "no custom filters",
			given:    []telemetryv1beta1.FluentBitFilter{},
			expected: nil,
		},
		{
			name: "filter with missing name",
			given: []telemetryv1beta1.FluentBitFilter{
				{Custom: "    Match   *"},
			},
			expected: []string{"unknown"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, CustomFilterPluginNames(test.given))
		})
	}
}
