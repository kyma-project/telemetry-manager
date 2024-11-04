package labels

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMakeDefaultLabel(t *testing.T) {
	podLabel := MakeDefaultLabel("my-pod")
	require.Equal(t, map[string]string{
		"app.kubernetes.io/name": "my-pod",
	}, podLabel)
}

func TestMakeMetricAgentSelectorLabel(t *testing.T) {
	metricAgentSelectorLabel := MakeMetricAgentSelectorLabel("metric-agent")
	require.Equal(t, map[string]string{
		"app.kubernetes.io/name":                  "metric-agent",
		"telemetry.kyma-project.io/metric-scrape": "true",
		"sidecar.istio.io/inject":                 "true",
	}, metricAgentSelectorLabel)
}

func TestMakeMetricGatewaySelectorLabel(t *testing.T) {
	metricGatewaySelectorLabel := MakeMetricGatewaySelectorLabel("metric-gateway")
	require.Equal(t, map[string]string{
		"app.kubernetes.io/name":                  "metric-gateway",
		"telemetry.kyma-project.io/metric-ingest": "true",
		"telemetry.kyma-project.io/metric-export": "true",
	}, metricGatewaySelectorLabel)
}
func TestMakeTraceGatewaySelectorLabel(t *testing.T) {
	traceGatewaySelectorLabel := MakeTraceGatewaySelectorLabel("trace-gateway")
	require.Equal(t, map[string]string{
		"app.kubernetes.io/name":                 "trace-gateway",
		"telemetry.kyma-project.io/trace-ingest": "true",
		"telemetry.kyma-project.io/trace-export": "true",
	}, traceGatewaySelectorLabel)
}
