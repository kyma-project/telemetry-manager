//go:build e2e

package matchers

import (
	"fmt"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"go.opentelemetry.io/collector/pdata/pmetric"

	kitmetrics "github.com/kyma-project/telemetry-manager/test/testkit/otlp/metrics"
)

// ContainMetrics succeeds if the filexporter output file contains the metrics passed into the matcher. The ordering of the elements does not matter.
func ContainMetrics(expectedMetrics ...pmetric.Metric) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlMetrics []byte) ([]pmetric.Metric, error) {
		mds, err := extractMetrics(jsonlMetrics)
		if err != nil {
			return nil, fmt.Errorf("ContainMetrics requires a valid OTLP JSON document: %v", err)
		}

		var metrics []pmetric.Metric
		for _, md := range mds {
			metrics = append(metrics, kitmetrics.AllMetrics(md)...)
		}
		return metrics, nil
	}, gomega.ContainElements(expectedMetrics))
}

type MetricPredicate = func(pmetric.Metric) bool

// ContainMetricsThatSatisfy succeeds if the filexporter output file contains metrics that satisfy the predicate passed into the matcher. The ordering of the elements does not matter.
func ContainMetricsThatSatisfy(predicate MetricPredicate) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlMetrics []byte) ([]pmetric.Metric, error) {
		mds, err := extractMetrics(jsonlMetrics)
		if err != nil {
			return nil, fmt.Errorf("ContainMetricsThatSatisfy requires a valid OTLP JSON document: %v", err)
		}

		var metrics []pmetric.Metric
		for _, md := range mds {
			metrics = append(metrics, kitmetrics.AllMetrics(md)...)
		}
		return metrics, nil
	}, gomega.ContainElements(gomega.Satisfy(predicate)))
}

// ConsistOfNumberOfMetrics succeeds if the filexporter output file has the expected number of metrics.
func ConsistOfNumberOfMetrics(expectedMetricCount int) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlMetrics []byte) (int, error) {
		mds, err := extractMetrics(jsonlMetrics)
		if err != nil {
			return 0, fmt.Errorf("ConsistOfNumberOfMetrics requires a valid OTLP JSON document: %v", err)
		}
		metricCount := 0
		for _, md := range mds {
			metricCount += len(kitmetrics.AllMetrics(md))
		}

		return metricCount, nil
	}, gomega.Equal(expectedMetricCount))
}

// ContainMetricsWithNames succeeds if the filexporter output file contains metrics with names passed into the matcher. The ordering of the elements does not matter.
func ContainMetricsWithNames(expectedMetricNames ...string) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlMetrics []byte) ([]string, error) {
		mds, err := extractMetrics(jsonlMetrics)
		if err != nil {
			return nil, fmt.Errorf("ContainMetricsWithNames requires a valid OTLP JSON document: %v", err)
		}

		var metricNames []string
		for _, md := range mds {
			metricNames = append(metricNames, kitmetrics.AllMetricNames(md)...)
		}

		return metricNames, nil
	}, gomega.ContainElements(expectedMetricNames))
}

func ConsistOfMetricsWithResourceAttributes(expectedAttributeNames ...string) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlMetrics []byte) ([][]string, error) {
		mds, err := extractMetrics(jsonlMetrics)
		if err != nil {
			return nil, fmt.Errorf("ConsistOfMetricsWithResourceAttributes requires a valid OTLP JSON document: %v", err)
		}

		var attributeNames [][]string
		for _, md := range mds {
			attributeNames = append(attributeNames, kitmetrics.AllResourceAttributeNames(md))
		}

		return attributeNames, nil
	}, gomega.HaveEach(gomega.ConsistOf(expectedAttributeNames)))
}

func extractMetrics(fileBytes []byte) ([]pmetric.Metrics, error) {
	mds, err := unmarshalMetrics(fileBytes)
	if err != nil {
		return nil, err
	}

	applyTemporalityWorkaround(mds)

	return mds, nil
}

func unmarshalMetrics(jsonlMetrics []byte) ([]pmetric.Metrics, error) {
	return unmarshalSignals[pmetric.Metrics](jsonlMetrics, func(buf []byte) (pmetric.Metrics, error) {
		var unmarshaler pmetric.JSONUnmarshaler
		return unmarshaler.UnmarshalMetrics(buf)
	})
}

// applyTemporalityWorkaround flips temporality os a Sum metric. The reason for that is the inconsistency
// between the metricdata package (https://github.com/open-telemetry/opentelemetry-go/blob/main/sdk/metric/metricdata/temporality.go)
// and the pmetric package (https://github.com/open-telemetry/opentelemetry-collector/blob/main/pdata/pmetric/aggregation_temporality.go)
func applyTemporalityWorkaround(mds []pmetric.Metrics) {
	for _, md := range mds {
		for _, metric := range kitmetrics.AllMetrics(md) {
			if metric.Type() != pmetric.MetricTypeSum {
				continue
			}

			if metric.Sum().AggregationTemporality() == pmetric.AggregationTemporalityCumulative {
				metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
			} else {
				metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			}
		}
	}
}
