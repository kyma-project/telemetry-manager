package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestMakeRules(t *testing.T) {
	rules := MakeRules()
	rulesYAML, err := yaml.Marshal(rules)
	require.NoError(t, err)

	goldenFilePath := filepath.Join("testdata", "rules.yaml")
	if testutils.ShouldUpdateGoldenFiles() {
		testutils.UpdateGoldenFileYAML(t, goldenFilePath, rulesYAML)
	}

	goldenFile, err := os.ReadFile(goldenFilePath)
	require.NoError(t, err, "failed to load golden file")
	require.Equal(t, string(goldenFile), string(rulesYAML))
}

func TestMatchesLogPipelineRule(t *testing.T) {
	tests := []struct {
		name               string
		labelSet           map[string]string
		unprefixedRuleName string
		pipelineName       string
		expectedResult     bool
	}{
		{
			name: "rule name matches and pipeline name matches",
			labelSet: map[string]string{
				"alertname":     "LogFluentBitBufferInUse",
				"pipeline_name": "testPipeline",
			},
			unprefixedRuleName: "FluentBitBufferInUse",
			pipelineName:       "testPipeline",
			expectedResult:     true,
		},
		{
			name: "rule name matches and pipeline name does not match",
			labelSet: map[string]string{
				"alertname":     "LogFluentBitBufferInUse",
				"pipeline_name": "testPipeline",
			},
			unprefixedRuleName: "testAlert",
			pipelineName:       "otherPipeline",
			expectedResult:     false,
		},
		{
			name: "rule name does not match and pipeline name matches",
			labelSet: map[string]string{
				"alertname":     "MetricAgentBufferFull",
				"pipeline_name": "testPipeline",
			},
			unprefixedRuleName: "AgentBufferFull",
			pipelineName:       "testPipeline",
			expectedResult:     false,
		},
		{
			name: "rule name matches and name label is missing",
			labelSet: map[string]string{
				"alertname": "LogFluentBitBufferInUse",
			},
			unprefixedRuleName: "FluentBitBufferInUse",
			pipelineName:       "testPipeline",
			expectedResult:     true,
		},
		{
			name: "rule name is RulesAny and name label is missing",
			labelSet: map[string]string{
				"alertname": "LogFluentBitBufferInUse",
			},
			unprefixedRuleName: RulesAny,
			pipelineName:       "testPipeline",
			expectedResult:     true,
		},
		{
			name: "rule name is RulesAny and name label is present but doesn't match prefix",
			labelSet: map[string]string{
				"alertname":     "LogFluentBitBufferInUse",
				"pipeline_name": "otherPipeline",
			},
			unprefixedRuleName: RulesAny,
			pipelineName:       "testPipeline",
			expectedResult:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := MatchesLogPipelineRule(test.labelSet, test.unprefixedRuleName, test.pipelineName)
			require.Equal(t, test.expectedResult, result)
		})
	}
}

func TestMatchesMetricPipelineRule(t *testing.T) {
	tests := []struct {
		name               string
		labelSet           map[string]string
		unprefixedRuleName string
		pipelineName       string
		expectedResult     bool
	}{
		{
			name: "rule name matches and pipeline name matches",
			labelSet: map[string]string{
				"alertname":     "MetricGatewayExporterSentData",
				"pipeline_name": "testPipeline",
			},
			unprefixedRuleName: "GatewayExporterSentData",
			pipelineName:       "testPipeline",
			expectedResult:     true,
		},
		{
			name: "rule name matches and pipeline name does not match",
			labelSet: map[string]string{
				"alertname":     "MetricGatewayExporterSentData",
				"pipeline_name": "testPipeline",
			},
			unprefixedRuleName: "GatewayExporterSentData",
			pipelineName:       "otherPipeline",
			expectedResult:     false,
		},
		{
			name: "rule name does not match and pipeline name matches",
			labelSet: map[string]string{
				"alertname":     "LogFluentBitBufferInUse",
				"pipeline_name": "testPipeline",
			},
			unprefixedRuleName: "MetricGatewayExporterSentData",
			pipelineName:       "testPipeline",
			expectedResult:     false,
		},
		{
			name: "rule name matches and name label is missing",
			labelSet: map[string]string{
				"alertname": "MetricGatewayExporterSentData",
			},
			unprefixedRuleName: "GatewayExporterSentData",
			pipelineName:       "testPipeline",
			expectedResult:     true,
		},
		{
			name: "rule name is RulesAny and name label is missing",
			labelSet: map[string]string{
				"alertname": "MetricGatewayExporterSentData",
			},
			unprefixedRuleName: RulesAny,
			pipelineName:       "testPipeline",
			expectedResult:     true,
		},
		{
			name: "rule name is RulesAny and name label is present but doesn't match prefix",
			labelSet: map[string]string{
				"alertname":     "MetricGatewayExporterSentData",
				"pipeline_name": "otherPipeline",
			},
			unprefixedRuleName: RulesAny,
			pipelineName:       "otlp/testPipeline",
			expectedResult:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := MatchesMetricPipelineRule(test.labelSet, test.unprefixedRuleName, test.pipelineName)
			require.Equal(t, test.expectedResult, result)
		})
	}
}

func TestMatchesTracePipelineRule(t *testing.T) {
	tests := []struct {
		name               string
		labelSet           map[string]string
		unprefixedRuleName string
		pipelineName       string
		expectedResult     bool
	}{
		{
			name: "rule name matches and pipeline name matches",
			labelSet: map[string]string{
				"alertname":     "TraceGatewayExporterSentData",
				"pipeline_name": "testPipeline",
			},
			unprefixedRuleName: "GatewayExporterSentData",
			pipelineName:       "testPipeline",
			expectedResult:     true,
		},
		{
			name: "rule name matches and pipeline name does not match",
			labelSet: map[string]string{
				"alertname":     "TraceGatewayExporterSentData",
				"pipeline_name": "testPipeline",
			},
			unprefixedRuleName: "GatewayExporterSentData",
			pipelineName:       "otherPipeline",
			expectedResult:     false,
		},
		{
			name: "rule name does not match and pipeline name matches",
			labelSet: map[string]string{
				"alertname":     "LogFluentBitBufferInUse",
				"pipeline_name": "testPipeline",
			},
			unprefixedRuleName: "TraceGatewayExporterSentData",
			pipelineName:       "testPipeline",
			expectedResult:     false,
		},
		{
			name: "rule name matches and name label is missing",
			labelSet: map[string]string{
				"alertname": "TraceGatewayExporterSentData",
			},
			unprefixedRuleName: "GatewayExporterSentData",
			pipelineName:       "testPipeline",
			expectedResult:     true,
		},
		{
			name: "rule name is RulesAny and name label is missing",
			labelSet: map[string]string{
				"alertname": "TraceGatewayExporterSentData",
			},
			unprefixedRuleName: RulesAny,
			pipelineName:       "testPipeline",
			expectedResult:     true,
		},
		{
			name: "rule name is RulesAny and name label is present but doesn't match prefix",
			labelSet: map[string]string{
				"alertname":     "TraceGatewayExporterSentData",
				"pipeline_name": "otherPipeline",
			},
			unprefixedRuleName: RulesAny,
			pipelineName:       "otlp/testPipeline",
			expectedResult:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := MatchesTracePipelineRule(test.labelSet, test.unprefixedRuleName, test.pipelineName)
			require.Equal(t, test.expectedResult, result)
		})
	}
}

func TestMatchesOtelLogPipelineRule(t *testing.T) {
	tests := []struct {
		name               string
		labelSet           map[string]string
		unprefixedRuleName string
		pipelineName       string
		expectedResult     bool
	}{
		{
			name: "rule name matches and pipeline name matches",
			labelSet: map[string]string{
				"alertname":     "LogGatewayExporterSentData",
				"pipeline_name": "testPipeline",
			},
			unprefixedRuleName: "GatewayExporterSentData",
			pipelineName:       "testPipeline",
			expectedResult:     true,
		},
		{
			name: "rule name matches and pipeline name does not match",
			labelSet: map[string]string{
				"alertname":     "LogGatewayExporterSentData",
				"pipeline_name": "testPipeline",
			},
			unprefixedRuleName: "GatewayExporterSentData",
			pipelineName:       "otherPipeline",
			expectedResult:     false,
		},
		{
			name: "rule name does not match and pipeline name matches",
			labelSet: map[string]string{
				"alertname":     "LogFluentBitBufferInUse",
				"pipeline_name": "testPipeline",
			},
			unprefixedRuleName: "GatewayExporterSentData",
			pipelineName:       "testPipeline",
			expectedResult:     false,
		},
		{
			name: "rule name matches and name label is missing",
			labelSet: map[string]string{
				"alertname": "LogGatewayExporterSentData",
			},
			unprefixedRuleName: "GatewayExporterSentData",
			pipelineName:       "testPipeline",
			expectedResult:     true,
		},
		{
			name: "rule name is RulesAny and name label is missing",
			labelSet: map[string]string{
				"alertname": "LogGatewayExporterSentData",
			},
			unprefixedRuleName: RulesAny,
			pipelineName:       "testPipeline",
			expectedResult:     true,
		},
		{
			name: "rule name is RulesAny and name label is present but doesn't match prefix",
			labelSet: map[string]string{
				"alertname":     "LogGatewayExporterSentData",
				"pipeline_name": "otherPipeline",
			},
			unprefixedRuleName: RulesAny,
			pipelineName:       "otlp/testPipeline",
			expectedResult:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := MatchesLogPipelineRule(test.labelSet, test.unprefixedRuleName, test.pipelineName)
			require.Equal(t, test.expectedResult, result)
		})
	}
}
