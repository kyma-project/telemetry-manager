package gateway

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDropIstioInternalMetrics(t *testing.T) {
	dropFilter := makeFilterToDropMetricsForTelemetryComponents()
	expectedDropMetricsWithSourceMetricAgent := "isMatch(\"name\", \"otel.*\") and HasAttrOnDatapoint( \"source_worklaod\", \"telemetry-metric-agent\")"
	expectedDropMetricsWithDestinationTraceGateway := "isMatch(\"name\", \"otel.*\") and HasAttrOnDatapoint( \"destination_workload\", \"telemetry-metric-gateway\")"
	expectedDropMetricsWithDestinationMetricGateway := "isMatch(\"name\", \"otel.*\") and HasAttrOnDatapoint( \"destination_workload\", \"telemetry-trace-collector\")"
	expectedDropFilter := fmt.Sprintf("(%s or %s or %s)", expectedDropMetricsWithSourceMetricAgent, expectedDropMetricsWithDestinationTraceGateway, expectedDropMetricsWithDestinationMetricGateway)
	require.Len(t, dropFilter.Metrics.Metric, 1)
	require.Equal(t, []string{expectedDropFilter}, dropFilter.Metrics.Metric)
}
