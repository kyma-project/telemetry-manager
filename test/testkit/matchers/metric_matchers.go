//go:build e2e

package matchers

import (
	"bufio"
	"bytes"
	"fmt"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/kyma-project/telemetry-manager/test/testkit/otlp/metrics"
)

// ContainMetrics succeeds if the filexporter output file contains the metrics passed into the matcher. The ordering of the elements does not matter.
func ContainMetrics(expectedMetrics ...pmetric.Metric) types.GomegaMatcher {
	return gomega.WithTransform(func(fileBytes []byte) ([]pmetric.Metric, error) {
		actualMds, err := extractMetricsData(fileBytes)
		if err != nil {
			return nil, fmt.Errorf("ContainMetrics requires a valid OTLP JSON document: %v", err)
		}

		var actualMetrics []pmetric.Metric
		for _, md := range actualMds {
			actualMetrics = append(actualMetrics, metrics.AllMetrics(md)...)
		}
		return actualMetrics, nil
	}, gomega.ContainElements(expectedMetrics))
}

type MetricPredicate = func(pmetric.Metric) bool

// ContainMetricsThatSatisfy succeeds if the filexporter output file contains metrics that satisfy the predicate passed into the matcher. The ordering of the elements does not matter.
func ContainMetricsThatSatisfy(predicate MetricPredicate) types.GomegaMatcher {
	return gomega.WithTransform(func(fileBytes []byte) ([]pmetric.Metric, error) {
		actualMds, err := extractMetricsData(fileBytes)
		if err != nil {
			return nil, fmt.Errorf("ContainMetricsThatSatisfy requires a valid OTLP JSON document: %v", err)
		}

		var actualMetrics []pmetric.Metric
		for _, md := range actualMds {
			actualMetrics = append(actualMetrics, metrics.AllMetrics(md)...)
		}
		return actualMetrics, nil
	}, gomega.ContainElements(gomega.Satisfy(predicate)))
}

// ConsistOfNumberOfMetrics succeeds if the filexporter output file has the expected number of metrics.
func ConsistOfNumberOfMetrics(expectedMetricCount int) types.GomegaMatcher {
	return gomega.WithTransform(func(fileBytes []byte) (int, error) {
		actualMds, err := extractMetricsData(fileBytes)
		if err != nil {
			return 0, fmt.Errorf("ConsistOfNumberOfMetrics requires a valid OTLP JSON document: %v", err)
		}
		metricsCount := 0
		for _, md := range actualMds {
			metricsCount += len(metrics.AllMetrics(md))
		}

		return metricsCount, nil
	}, gomega.Equal(expectedMetricCount))
}

// ContainMetricsWithNames succeeds if the filexporter output file contains metrics with names passed into the matcher. The ordering of the elements does not matter.
func ContainMetricsWithNames(expectedMetricNames ...string) types.GomegaMatcher {
	return gomega.WithTransform(func(fileBytes []byte) ([]string, error) {
		actualMds, err := extractMetricsData(fileBytes)
		if err != nil {
			return nil, fmt.Errorf("ContainMetricsWithNames requires a valid OTLP JSON document: %v", err)
		}

		var actualMetricNames []string
		for _, md := range actualMds {
			actualMetricNames = append(actualMetricNames, metrics.AllMetricNames(md)...)
		}

		return actualMetricNames, nil
	}, gomega.ContainElements(expectedMetricNames))
}

func ConsistOfMetricsWithResourceAttributes(expectedAttributeNames ...string) types.GomegaMatcher {
	return gomega.WithTransform(func(fileBytes []byte) ([][]string, error) {
		actualMds, err := extractMetricsData(fileBytes)
		if err != nil {
			return nil, fmt.Errorf("ConsistOfMetricsWithResourceAttributes requires a valid OTLP JSON document: %v", err)
		}

		var actualAttributeNames [][]string
		for _, md := range actualMds {
			actualAttributeNames = append(actualAttributeNames, metrics.AllResourceAttributeNames(md))
		}

		return actualAttributeNames, nil
	}, gomega.HaveEach(gomega.ConsistOf(expectedAttributeNames)))
}

func extractMetricsData(fileBytes []byte) ([]pmetric.Metrics, error) {
	actualMds, err := unmarshalOTLPJSONMetrics(fileBytes)
	if err != nil {
		return nil, err
	}

	applyTemporalityWorkaround(actualMds)

	return actualMds, nil
}

func unmarshalOTLPJSONMetrics(buf []byte) ([]pmetric.Metrics, error) {
	var results []pmetric.Metrics

	var metricsUnmarshaler pmetric.JSONUnmarshaler
	scanner := bufio.NewScanner(bytes.NewReader(buf))
	// default buffer size causing 'token too long' error, buffer size configured for current test scenarios
	scannerBuffer := make([]byte, 0, 64*1024)
	scanner.Buffer(scannerBuffer, 1024*1024)
	for scanner.Scan() {
		td, err := metricsUnmarshaler.UnmarshalMetrics(scanner.Bytes())
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshall metrics: %v", err)
		}

		results = append(results, td)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read metrics: %v", err)
	}

	return results, nil
}

// applyTemporalityWorkaround flips temporality os a Sum metric. The reason for that is the inconsistency
// between the metricdata package (https://github.com/open-telemetry/opentelemetry-go/blob/main/sdk/metric/metricdata/temporality.go)
// and the pmetric package (https://github.com/open-telemetry/opentelemetry-collector/blob/main/pdata/pmetric/aggregation_temporality.go)
func applyTemporalityWorkaround(mds []pmetric.Metrics) {
	for _, md := range mds {
		for _, metric := range metrics.AllMetrics(md) {
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
