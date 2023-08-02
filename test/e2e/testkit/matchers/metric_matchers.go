//go:build e2e

package matchers

import (
	"bufio"
	"bytes"
	"fmt"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/kyma-project/telemetry-manager/test/e2e/testkit/otlp/metrics"
)

func HaveMetrics(expectedMetrics ...pmetric.Metric) types.GomegaMatcher {
	return gomega.WithTransform(func(fileBytes []byte) ([]pmetric.Metric, error) {
		actualMds, err := extractMetricsData(fileBytes)
		if err != nil {
			return nil, fmt.Errorf("HaveMetrics requires a valid OTLP JSON document: %v", err)
		}

		var actualMetrics []pmetric.Metric
		for _, md := range actualMds {
			actualMetrics = append(actualMetrics, metrics.AllMetrics(md)...)
		}
		return actualMetrics, nil
	}, gomega.ContainElements(expectedMetrics))
}

type MetricPredicate = func(pmetric.Metric) bool

func HaveMetricsThatSatisfy(predicate MetricPredicate) types.GomegaMatcher {
	return gomega.WithTransform(func(fileBytes []byte) ([]pmetric.Metric, error) {
		actualMds, err := extractMetricsData(fileBytes)
		if err != nil {
			return nil, fmt.Errorf("HaveMetricsThatSatisfy requires a valid OTLP JSON document: %v", err)
		}

		var actualMetrics []pmetric.Metric
		for _, md := range actualMds {
			actualMetrics = append(actualMetrics, metrics.AllMetrics(md)...)
		}
		return actualMetrics, nil
	}, gomega.ContainElements(gomega.Satisfy(predicate)))
}

func HaveNumberOfMetrics(expectedMetricCount int) types.GomegaMatcher {
	return gomega.WithTransform(func(fileBytes []byte) (int, error) {
		actualMds, err := extractMetricsData(fileBytes)
		if err != nil {
			return 0, fmt.Errorf("HaveNumberOfMetrics requires a valid OTLP JSON document: %v", err)
		}
		metricsCount := 0
		for _, md := range actualMds {
			metricsCount += len(metrics.AllMetrics(md))
		}

		return metricsCount, nil
	}, gomega.Equal(expectedMetricCount))
}

func HaveMetricNames(expectedMetricNames ...string) types.GomegaMatcher {
	return gomega.WithTransform(func(fileBytes []byte) ([]string, error) {
		actualMds, err := extractMetricsData(fileBytes)
		if err != nil {
			return nil, fmt.Errorf("HaveMetricNames requires a valid OTLP JSON document: %v", err)
		}

		var actualMetricNames []string
		for _, md := range actualMds {
			actualMetricNames = append(actualMetricNames, metrics.AllMetricNames(md)...)
		}

		return actualMetricNames, nil
	}, gomega.ContainElements(expectedMetricNames))
}

func HaveAttributes(expectedAttributeNames ...string) types.GomegaMatcher {
	return gomega.WithTransform(func(fileBytes []byte) ([]string, error) {
		actualMds, err := extractMetricsData(fileBytes)
		if err != nil {
			return nil, fmt.Errorf("HaveAttributes requires a valid OTLP JSON document: %v", err)
		}

		var actualAttributeNames []string
		for _, md := range actualMds {
			actualAttributeNames = append(actualAttributeNames, metrics.AllResourceAttributeNames(md)...)
		}

		return actualAttributeNames, nil
	}, gomega.ContainElements(expectedAttributeNames))
}

func extractMetricsData(fileBytes []byte) ([]pmetric.Metrics, error) {
	actualMds, err := unmarshalOTLPJSONMetrics(fileBytes)
	if err != nil {
		return nil, err
	}

	applyAggregationWorkaround(actualMds)

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

func applyAggregationWorkaround(mds []pmetric.Metrics) {
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
