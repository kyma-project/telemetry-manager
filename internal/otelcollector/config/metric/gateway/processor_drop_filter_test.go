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

var k8sClusterMetricsDrop = []string{"instrumentation_scope.name == \"io.kyma-project.telemetry/k8s_cluster\"" +
	" and (IsMatch(name, \"^k8s.deployment.*\") or IsMatch(name, \"^k8s.cronjob.*\") or IsMatch(name, \"^k8s.daemonset.*\") or IsMatch(name, \"^k8s.hpa.*\") or IsMatch(name, \"^k8s.job.*\")" +
	" or IsMatch(name, \"^k8s.namespace.*\") or IsMatch(name, \"^k8s.replicaset.*\") or IsMatch(name, \"^k8s.replication_controller.*\") or IsMatch(name, \"^k8s.resource_quota.*\") or IsMatch(name, \"^k8s.statefulset.*\")" +
	" or IsMatch(name, \"^openshift.*\") or IsMatch(name, \"^k8s.node.*\"))"}

func TestMakeK8sClusterDropMetrics(t *testing.T) {
	dropFilter := makeK8sClusterDropMetrics()
	require.Len(t, dropFilter.Metrics.Metric, 1)
	require.Equal(t, k8sClusterMetricsDrop, dropFilter.Metrics.Metric)
}
