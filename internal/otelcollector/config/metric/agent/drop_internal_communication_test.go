package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDropIstioInternalMetrics(t *testing.T) {
	dropFilter := makeFilterToDropMetricsForTelemetryComponents()
	expectedDropMetricsWithSourceMetricAgent := "IsMatch(name, \"istio.*\") and HasAttrOnDatapoint(\"source_workload\", \"telemetry-metric-agent\")"
	expectedDropMetricsWithDestinationTraceGateway := "IsMatch(name, \"istio.*\") and HasAttrOnDatapoint(\"destination_workload\", \"telemetry-metric-gateway\")"
	expectedDropMetricsWithDestinationMetricGateway := "IsMatch(name, \"istio.*\") and HasAttrOnDatapoint(\"destination_workload\", \"telemetry-trace-collector\")"
	require.Len(t, dropFilter.Metrics.Metric, 3)
	require.Equal(t, []string{expectedDropMetricsWithSourceMetricAgent, expectedDropMetricsWithDestinationTraceGateway, expectedDropMetricsWithDestinationMetricGateway}, dropFilter.Metrics.Metric)
}
