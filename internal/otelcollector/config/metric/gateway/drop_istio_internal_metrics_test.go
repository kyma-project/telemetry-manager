package gateway

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDropIstioInternalMetrics(t *testing.T) {
	dropFilter := makeFilterToDropMetricsForTelemetryComponents()
	expectedDropMetricsWithSourceMetricAgent := "IsMatch(name, \"istio.*\") and HasAttrOnDatapoint(\"source_workload\", \"telemetry-metric-agent\")"
	expectedDropMetricsWithDestinationTraceGateway := "IsMatch(name, \"istio.*\") and HasAttrOnDatapoint(\"destination_workload\", \"telemetry-metric-gateway\")"
	expectedDropMetricsWithDestinationMetricGateway := "IsMatch(name, \"istio.*\") and HasAttrOnDatapoint(\"destination_workload\", \"telemetry-trace-collector\")"
	//expectedDropFilter := fmt.Sprintf("(%s or %s or %s)", expectedDropMetricsWithSourceMetricAgent, expectedDropMetricsWithDestinationTraceGateway, expectedDropMetricsWithDestinationMetricGateway)
	require.Len(t, dropFilter.Metrics.Metric, 3)
	require.Equal(t, []string{expectedDropMetricsWithSourceMetricAgent, expectedDropMetricsWithDestinationTraceGateway, expectedDropMetricsWithDestinationMetricGateway}, dropFilter.Metrics.Metric)
}
