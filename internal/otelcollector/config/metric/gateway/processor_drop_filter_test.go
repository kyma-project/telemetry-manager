package gateway

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDropDiagnosticMetricsForInput(t *testing.T) {
	dropFilterIstio := makeDropDiagnosticMetricsForInput(inputSourceEquals("istio"))
	dropFilterPrometheus := makeDropDiagnosticMetricsForInput(inputSourceEquals("prometheus"))
	expectedDropMetricsIstioConfiguration := "instrumentation_scope.name == \"io.kyma-project.telemetry/istio\" and (name == \"up\" or name == \"scrape_duration_seconds\" or name == \"scrape_samples_scraped\" or name == \"scrape_samples_post_metric_relabeling\" or name == \"scrape_series_added\")"

	require.Len(t, dropFilterIstio.Metrics.Metric, 1)
	require.Equal(t, []string{expectedDropMetricsIstioConfiguration}, dropFilterIstio.Metrics.Metric)

	expectedDropMetricsPrometheusConfiguration := "instrumentation_scope.name == \"io.kyma-project.telemetry/prometheus\" and (name == \"up\" or name == \"scrape_duration_seconds\" or name == \"scrape_samples_scraped\" or name == \"scrape_samples_post_metric_relabeling\" or name == \"scrape_series_added\")"

	require.Len(t, dropFilterPrometheus.Metrics.Metric, 1)
	require.Equal(t, []string{expectedDropMetricsPrometheusConfiguration}, dropFilterPrometheus.Metrics.Metric)
}
