package flowhealth

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMakeRules(t *testing.T) {
	rules := MakeRules()

	require.Len(t, rules.Groups, 1)

	ruleGroup := rules.Groups[0]
	require.Equal(t, "default", ruleGroup.Name)

	require.Len(t, ruleGroup.Rules, 9)
	require.Equal(t, "GatewayExporterSentMetricPoints", ruleGroup.Rules[0].Alert)
	require.Equal(t, "GatewayExporterSentSpans", ruleGroup.Rules[1].Alert)
	require.Equal(t, "GatewayExporterDroppedMetricPoints", ruleGroup.Rules[2].Alert)
	require.Equal(t, "GatewayExporterDroppedSpans", ruleGroup.Rules[3].Alert)
	require.Equal(t, "GatewayExporterQueueAlmostFull", ruleGroup.Rules[4].Alert)
	require.Equal(t, "GatewayReceiverRefusedMetricPoints", ruleGroup.Rules[5].Alert)
	require.Equal(t, "GatewayReceiverRefusedSpans", ruleGroup.Rules[6].Alert)
	require.Equal(t, "GatewayExporterEnqueueFailedMetricPoints", ruleGroup.Rules[7].Alert)
	require.Equal(t, "GatewayExporterEnqueueFailedSpans", ruleGroup.Rules[8].Alert)

	require.Equal(t, "sum by (exporter) (rate(otelcol_exporter_sent_metric_points{service=\"telemetry-metric-gateway-metrics\"}[1m])) > 0", ruleGroup.Rules[0].Expr)
	require.Equal(t, "sum by (exporter) (rate(otelcol_exporter_sent_spans{service=\"telemetry-trace-gateway-metrics\"}[1m])) > 0", ruleGroup.Rules[1].Expr)
	require.Equal(t, "sum by (exporter) (rate(otelcol_exporter_send_failed_metric_points{service=\"telemetry-metric-gateway-metrics\"}[1m])) > 0", ruleGroup.Rules[2].Expr)
	require.Equal(t, "sum by (exporter) (rate(otelcol_exporter_send_failed_spans{service=\"telemetry-trace-gateway-metrics\"}[1m])) > 0", ruleGroup.Rules[3].Expr)
	require.Equal(t, "otelcol_exporter_queue_size / otelcol_exporter_queue_capacity > 0.8", ruleGroup.Rules[4].Expr)
	require.Equal(t, "sum by (receiver) (rate(otelcol_receiver_refused_metric_points{service=\"telemetry-metric-gateway-metrics\"}[1m])) > 0", ruleGroup.Rules[5].Expr)
	require.Equal(t, "sum by (receiver) (rate(otelcol_receiver_refused_spans{service=\"telemetry-trace-gateway-metrics\"}[1m])) > 0", ruleGroup.Rules[6].Expr)
	require.Equal(t, "sum by (exporter) (rate(otelcol_exporter_enqueue_failed_metric_points{service=\"telemetry-metric-gateway-metrics\"}[1m])) > 0", ruleGroup.Rules[7].Expr)
	require.Equal(t, "sum by (exporter) (rate(otelcol_exporter_enqueue_failed_spans{service=\"telemetry-trace-gateway-metrics\"}[1m])) > 0", ruleGroup.Rules[8].Expr)
}
