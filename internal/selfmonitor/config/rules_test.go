package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMakeRules(t *testing.T) {
	rules := MakeRules()

	require.Len(t, rules.Groups, 1)

	ruleGroup := rules.Groups[0]
	require.Equal(t, "default", ruleGroup.Name)

	require.Len(t, ruleGroup.Rules, 15)
	require.Equal(t, "MetricGatewayExporterSentData", ruleGroup.Rules[0].Alert)
	require.Equal(t, "sum by (pipeline_name) (rate(otelcol_exporter_sent_metric_points{service=\"telemetry-metric-gateway-metrics\"}[5m])) > 0", ruleGroup.Rules[0].Expr)

	require.Equal(t, "MetricGatewayExporterDroppedData", ruleGroup.Rules[1].Alert)
	require.Equal(t, "sum by (pipeline_name) (rate(otelcol_exporter_send_failed_metric_points{service=\"telemetry-metric-gateway-metrics\"}[5m])) > 0", ruleGroup.Rules[1].Expr)

	require.Equal(t, "MetricGatewayExporterQueueAlmostFull", ruleGroup.Rules[2].Alert)
	require.Equal(t, "max by (pipeline_name) (otelcol_exporter_queue_size{service=\"telemetry-metric-gateway-metrics\"} / ignoring(data_type) otelcol_exporter_queue_capacity{service=\"telemetry-metric-gateway-metrics\"}) > 0.8", ruleGroup.Rules[2].Expr)

	require.Equal(t, "MetricGatewayExporterEnqueueFailed", ruleGroup.Rules[3].Alert)
	require.Equal(t, "sum by (pipeline_name) (rate(otelcol_exporter_enqueue_failed_metric_points{service=\"telemetry-metric-gateway-metrics\"}[5m])) > 0", ruleGroup.Rules[3].Expr)

	require.Equal(t, "MetricGatewayReceiverRefusedData", ruleGroup.Rules[4].Alert)
	require.Equal(t, "sum by (receiver) (rate(otelcol_receiver_refused_metric_points{service=\"telemetry-metric-gateway-metrics\"}[5m])) > 0", ruleGroup.Rules[4].Expr)

	require.Equal(t, "TraceGatewayExporterSentData", ruleGroup.Rules[5].Alert)
	require.Equal(t, "sum by (pipeline_name) (rate(otelcol_exporter_sent_spans{service=\"telemetry-trace-collector-metrics\"}[5m])) > 0", ruleGroup.Rules[5].Expr)

	require.Equal(t, "TraceGatewayExporterDroppedData", ruleGroup.Rules[6].Alert)
	require.Equal(t, "sum by (pipeline_name) (rate(otelcol_exporter_send_failed_spans{service=\"telemetry-trace-collector-metrics\"}[5m])) > 0", ruleGroup.Rules[6].Expr)

	require.Equal(t, "TraceGatewayExporterQueueAlmostFull", ruleGroup.Rules[7].Alert)
	require.Equal(t, "max by (pipeline_name) (otelcol_exporter_queue_size{service=\"telemetry-trace-collector-metrics\"} / ignoring(data_type) otelcol_exporter_queue_capacity{service=\"telemetry-trace-collector-metrics\"}) > 0.8", ruleGroup.Rules[7].Expr)

	require.Equal(t, "TraceGatewayExporterEnqueueFailed", ruleGroup.Rules[8].Alert)
	require.Equal(t, "sum by (pipeline_name) (rate(otelcol_exporter_enqueue_failed_spans{service=\"telemetry-trace-collector-metrics\"}[5m])) > 0", ruleGroup.Rules[8].Expr)

	require.Equal(t, "TraceGatewayReceiverRefusedData", ruleGroup.Rules[9].Alert)
	require.Equal(t, "sum by (receiver) (rate(otelcol_receiver_refused_spans{service=\"telemetry-trace-collector-metrics\"}[5m])) > 0", ruleGroup.Rules[9].Expr)

	require.Equal(t, "LogAgentExporterSentLogs", ruleGroup.Rules[10].Alert)
	require.Equal(t, "sum by (pipeline_name) (rate(fluentbit_output_proc_bytes_total{service=\"telemetry-fluent-bit-metrics\"}[5m])) > 0", ruleGroup.Rules[10].Expr)

	require.Equal(t, "LogAgentExporterDroppedLogs", ruleGroup.Rules[11].Alert)
	require.Equal(t, "sum by (pipeline_name) (rate(fluentbit_output_dropped_records_total{service=\"telemetry-fluent-bit-metrics\"}[5m])) > 0", ruleGroup.Rules[11].Expr)

	require.Equal(t, "LogAgentBufferInUse", ruleGroup.Rules[12].Alert)
	require.Equal(t, "telemetry_fsbuffer_usage_bytes{service=\"telemetry-fluent-bit-exporter-metrics\"} > 300000000", ruleGroup.Rules[12].Expr)

	require.Equal(t, "LogAgentBufferFull", ruleGroup.Rules[13].Alert)
	require.Equal(t, "telemetry_fsbuffer_usage_bytes{service=\"telemetry-fluent-bit-exporter-metrics\"} > 900000000", ruleGroup.Rules[13].Expr)

	require.Equal(t, "LogAgentNoLogsDelivered", ruleGroup.Rules[14].Alert)
	require.Equal(t, "(sum by (pipeline_name) (rate(fluentbit_input_bytes_total{service=\"telemetry-fluent-bit-metrics\"}[5m])) > 0) and (sum by (pipeline_name) (rate(fluentbit_output_proc_bytes_total{service=\"telemetry-fluent-bit-metrics\"}[5m])) == 0)", ruleGroup.Rules[14].Expr)
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
				"alertname":     "LogAgentBufferFull",
				"pipeline_name": "testPipeline",
			},
			unprefixedRuleName: "AgentBufferFull",
			pipelineName:       "testPipeline",
			expectedResult:     true,
		},
		{
			name: "rule name matches and pipeline name does not match",
			labelSet: map[string]string{
				"alertname":     "LogAgentBufferFull",
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
				"alertname": "LogAgentBufferFull",
			},
			unprefixedRuleName: "AgentBufferFull",
			pipelineName:       "testPipeline",
			expectedResult:     true,
		},
		{
			name: "rule name is RulesAny and name label is missing",
			labelSet: map[string]string{
				"alertname": "LogAgentBufferFull",
			},
			unprefixedRuleName: RulesAny,
			pipelineName:       "testPipeline",
			expectedResult:     true,
		},
		{
			name: "rule name is RulesAny and name label is present but doesn't match prefix",
			labelSet: map[string]string{
				"alertname":     "LogAgentBufferFull",
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
				"alertname":     "LogAgentBufferFull",
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
				"alertname":     "LogAgentBufferFull",
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
