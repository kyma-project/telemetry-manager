package agent

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/ottlexpr"
)

var (
	dropMetricsWithSourceMetricAgent        = ottlexpr.JoinWithAnd(ottlexpr.IsMatch("name", "istio.*"), ottlexpr.HasAttrOnDatapoint("source_workload", "telemetry-metric-agent"))
	dropMetricsWithDestinationTraceGateway  = ottlexpr.JoinWithAnd(ottlexpr.IsMatch("name", "istio.*"), ottlexpr.HasAttrOnDatapoint("destination_workload", "telemetry-metric-gateway"))
	dropMetricsWithDestinationMetricGateway = ottlexpr.JoinWithAnd(ottlexpr.IsMatch("name", "istio.*"), ottlexpr.HasAttrOnDatapoint("destination_workload", "telemetry-trace-collector"))
)

func makeFilterToDropMetricsForTelemetryComponents() *FilterProcessor {

	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				dropMetricsWithSourceMetricAgent,
				dropMetricsWithDestinationTraceGateway,
				dropMetricsWithDestinationMetricGateway,
			},
		},
	}
}
