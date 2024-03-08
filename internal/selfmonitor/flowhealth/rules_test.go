package flowhealth

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMakeRules(t *testing.T) {
	rules := MakeRules()

	require.Len(t, rules.Groups, 1)
	require.Equal(t, "default", rules.Groups[0].Name)

	require.Len(t, rules.Groups[0].Rules, 9)
	require.ElementsMatch(t, []Rule{
		{
			Alert: "GatewayExporterSentMetricPoints",
			Expr:  "sum by (exporter) (rate(otelcol_exporter_sent_metric_points{service=\"telemetry-metric-gateway-metrics\"}[1m])) > 0",
		},
		{
			Alert: "GatewayExporterSentSpans",
			Expr:  "sum by (exporter) (rate(otelcol_exporter_sent_spans{service=\"telemetry-trace-gateway-metrics\"}[1m])) > 0",
		},
		{
			Alert: "GatewayExporterDroppedMetricPoints",
			Expr:  "sum by (exporter) (rate(otelcol_exporter_send_failed_metric_points{service=\"telemetry-metric-gateway-metrics\"}[1m])) > 0",
		},
		{
			Alert: "GatewayExporterDroppedSpans",
			Expr:  "sum by (exporter) (rate(otelcol_exporter_send_failed_spans{service=\"telemetry-trace-gateway-metrics\"}[1m])) > 0",
		},
		{
			Alert: "GatewayExporterQueueAlmostFull",
			Expr:  "otelcol_exporter_queue_size / otelcol_exporter_queue_capacity > 0.8",
		},
		{
			Alert: "GatewayReceiverRefusedMetricPoints",
			Expr:  "sum by (receiver) (rate(otelcol_receiver_refused_metric_points{service=\"telemetry-metric-gateway-metrics\"}[1m])) > 0",
		},
		{
			Alert: "GatewayReceiverRefusedSpans",
			Expr:  "sum by (receiver) (rate(otelcol_receiver_refused_spans{service=\"telemetry-trace-gateway-metrics\"}[1m])) > 0",
		},
		{
			Alert: "GatewayExporterEnqueueFailedMetricPoints",
			Expr:  "sum by (exporter) (rate(otelcol_exporter_enqueue_failed_metric_points{service=\"telemetry-metric-gateway-metrics\"}[1m])) > 0",
		},
		{
			Alert: "GatewayExporterEnqueueFailedSpans",
			Expr:  "sum by (exporter) (rate(otelcol_exporter_enqueue_failed_spans{service=\"telemetry-trace-gateway-metrics\"}[1m])) > 0",
		},
	}, rules.Groups[0].Rules)
}
