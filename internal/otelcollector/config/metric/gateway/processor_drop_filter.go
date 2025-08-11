package gateway

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/ottlexpr"
)

var scrapeMetrics = []string{"up", "scrape_duration_seconds", "scrape_samples_scraped", "scrape_samples_post_metric_relabeling", "scrape_series_added"}

func dropDiagnosticMetricsFilterConfig(inputSourceCondition string) *FilterProcessor {
	var filterExpressions []string

	metricNameConditions := nameConditions(scrapeMetrics)
	excludeScrapeMetricsExpr := ottlexpr.JoinWithAnd(inputSourceCondition, ottlexpr.JoinWithOr(metricNameConditions...))
	filterExpressions = append(filterExpressions, excludeScrapeMetricsExpr)

	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: filterExpressions,
		},
	}
}

func nameConditions(names []string) []string {
	var nameConditions []string
	for _, name := range names {
		nameConditions = append(nameConditions, ottlexpr.NameAttributeEquals(name))
	}

	return nameConditions
}
