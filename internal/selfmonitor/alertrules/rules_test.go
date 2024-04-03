package alertrules

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMakeRules(t *testing.T) {
	rules := MakeRules()

	require.Len(t, rules.Groups, 1)

	ruleGroup := rules.Groups[0]
	require.Equal(t, "default", ruleGroup.Name)

	require.Len(t, ruleGroup.Rules, 10)
	require.Equal(t, "MetricGatewayExporterSentData", ruleGroup.Rules[0].Alert)
	require.Equal(t, "sum by (exporter) (rate(otelcol_exporter_sent_metric_points{service=\"telemetry-metric-gateway-metrics\"}[5m])) > 0", ruleGroup.Rules[0].Expr)

	require.Equal(t, "MetricGatewayExporterDroppedData", ruleGroup.Rules[1].Alert)
	require.Equal(t, "sum by (exporter) (rate(otelcol_exporter_send_failed_metric_points{service=\"telemetry-metric-gateway-metrics\"}[5m])) > 0", ruleGroup.Rules[1].Expr)

	require.Equal(t, "MetricGatewayExporterQueueAlmostFull", ruleGroup.Rules[2].Alert)
	require.Equal(t, "otelcol_exporter_queue_size{service=\"telemetry-metric-gateway-metrics\"} / otelcol_exporter_queue_capacity{service=\"telemetry-metric-gateway-metrics\"} > 0.8", ruleGroup.Rules[2].Expr)

	require.Equal(t, "MetricGatewayExporterEnqueueFailed", ruleGroup.Rules[3].Alert)
	require.Equal(t, "sum by (exporter) (rate(otelcol_exporter_enqueue_failed_metric_points{service=\"telemetry-metric-gateway-metrics\"}[5m])) > 0", ruleGroup.Rules[3].Expr)

	require.Equal(t, "MetricGatewayReceiverRefusedData", ruleGroup.Rules[4].Alert)
	require.Equal(t, "sum by (receiver) (rate(otelcol_receiver_refused_metric_points{service=\"telemetry-metric-gateway-metrics\"}[5m])) > 0", ruleGroup.Rules[4].Expr)

	require.Equal(t, "TraceGatewayExporterSentData", ruleGroup.Rules[5].Alert)
	require.Equal(t, "sum by (exporter) (rate(otelcol_exporter_sent_spans{service=\"telemetry-trace-collector-metrics\"}[5m])) > 0", ruleGroup.Rules[5].Expr)

	require.Equal(t, "TraceGatewayExporterDroppedData", ruleGroup.Rules[6].Alert)
	require.Equal(t, "sum by (exporter) (rate(otelcol_exporter_send_failed_spans{service=\"telemetry-trace-collector-metrics\"}[5m])) > 0", ruleGroup.Rules[6].Expr)

	require.Equal(t, "TraceGatewayExporterQueueAlmostFull", ruleGroup.Rules[7].Alert)
	require.Equal(t, "otelcol_exporter_queue_size{service=\"telemetry-trace-collector-metrics\"} / otelcol_exporter_queue_capacity{service=\"telemetry-trace-collector-metrics\"} > 0.8", ruleGroup.Rules[7].Expr)

	require.Equal(t, "TraceGatewayExporterEnqueueFailed", ruleGroup.Rules[8].Alert)
	require.Equal(t, "sum by (exporter) (rate(otelcol_exporter_enqueue_failed_spans{service=\"telemetry-trace-collector-metrics\"}[5m])) > 0", ruleGroup.Rules[8].Expr)

	require.Equal(t, "TraceGatewayReceiverRefusedData", ruleGroup.Rules[9].Alert)
	require.Equal(t, "sum by (receiver) (rate(otelcol_receiver_refused_spans{service=\"telemetry-trace-collector-metrics\"}[5m])) > 0", ruleGroup.Rules[9].Expr)
}
