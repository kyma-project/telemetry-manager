package gateway

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/ottlexpr"
)

var scrapeMetrics = []string{"up", "scrape_duration_seconds", "scrape_samples_scraped", "scrape_samples_post_metric_relabeling", "scrape_series_added"}

func makeDropDiagnosticMetricsForInput(inputSourceCondition string) *FilterProcessor {
	var filterExpressions []string
	metricNameConditions := createNameConditions(scrapeMetrics)
	excludeScrapeMetricsExpr := ottlexpr.JoinWithAnd(inputSourceCondition, ottlexpr.JoinWithOr(metricNameConditions...))
	filterExpressions = append(filterExpressions, excludeScrapeMetricsExpr)

	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: filterExpressions,
		},
	}
}

// Drop the metrics scraped by k8s cluster, except for the pod and container metrics
// Complete list of the metrics is here: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/k8sclusterreceiver/metadata.yaml
func makeK8sClusterDropMetrics() *FilterProcessor {
	metricNames := []string{
		"^k8s.deployment.*",
		"^k8s.cronjob.*",
		"^k8s.daemonset.*",
		"^k8s.hpa.*",
		"^k8s.job.*",
		"^k8s.namespace.*",
		"^k8s.replicaset.*",
		"^k8s.replication_controller.*",
		"^k8s.resource_quota.*",
		"^k8s.statefulset.*",
		"^openshift.*",
		"^k8s.node.*",
	}
	metricNameConditions := createIsMatchNameConditions(metricNames)
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.JoinWithAnd(
					inputSourceEquals(metric.InputSourceK8sCluster),
					ottlexpr.JoinWithOr(metricNameConditions...),
				),
			},
		},
	}
}

func createIsMatchNameConditions(names []string) []string {
	var nameConditions []string
	for _, name := range names {
		nameConditions = append(nameConditions, ottlexpr.IsMatch("name", name))
	}
	return nameConditions
}

func createNameConditions(names []string) []string {
	var nameConditions []string
	for _, name := range names {
		nameConditions = append(nameConditions, ottlexpr.NameAttributeEquals(name))
	}
	return nameConditions
}
