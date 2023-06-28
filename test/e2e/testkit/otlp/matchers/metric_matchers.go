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
	return gomega.WithTransform(func(actual interface{}) ([]pmetric.Metric, error) {
		actualBytes, ok := actual.([]byte)
		if !ok {
			return nil, fmt.Errorf("HaveGauges requires a []byte, but got %T", actual)
		}

		actualMds, err := unmarshalOTLPJSONMetrics(actualBytes)
		if err != nil {
			return nil, fmt.Errorf("HaveGauges requires a valid OTLP JSON document: %v", err)
		}

		var actualMetrics []pmetric.Metric
		for _, md := range actualMds {
			actualMetrics = append(actualMetrics, metrics.AllMetrics(md)...)
		}
		return actualMetrics, nil
	}, gomega.ContainElements(expectedMetrics))
}

func HaveDeltaMetrics(expectedMetrics ...pmetric.Metric) types.GomegaMatcher {
	return gomega.WithTransform(func(actual interface{}) ([]pmetric.Metric, error) {
		actualBytes, ok := actual.([]byte)
		if !ok {
			return nil, fmt.Errorf("HaveGauges requires a []byte, but got %T", actual)
		}
		fmt.Println(string(actualBytes))

		actualMds, err := unmarshalOTLPJSONMetrics(actualBytes)
		if err != nil {
			return nil, fmt.Errorf("HaveGauges requires a valid OTLP JSON document: %v", err)
		}
		fmt.Println(actualMds[len(actualMds)-1].ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().SchemaUrl())

		var actualMetrics []pmetric.Metric
		for _, md := range actualMds {
			actualMetrics = append(actualMetrics, metrics.AllMetrics(md)...)
		}
		fmt.Println(actualMetrics[len(actualMetrics)-1].Sum().AggregationTemporality())
		fmt.Println(actualMetrics[len(actualMetrics)-1].Sum().DataPoints().At(0))
		fmt.Println(actualMetrics[len(actualMetrics)-1].Sum().DataPoints().At(1))
		fmt.Println(actualMetrics[len(actualMetrics)-1].Sum().DataPoints().At(2))
		return actualMetrics, nil
	}, gomega.ContainElements(expectedMetrics))
}

func HaveNumberOfMetrics(expectedMetricCount int) types.GomegaMatcher {
	return gomega.WithTransform(func(actual interface{}) (int, error) {
		actualBytes, ok := actual.([]byte)
		if !ok {
			return 0, fmt.Errorf("HaveNumberOfMetrics requires a []byte, but got %T", actual)
		}

		actualMds, err := unmarshalOTLPJSONMetrics(actualBytes)
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

func unmarshalOTLPJSONMetrics(buf []byte) ([]pmetric.Metrics, error) {
	var results []pmetric.Metrics

	var metricsUnmarshaler pmetric.JSONUnmarshaler
	scanner := bufio.NewScanner(bytes.NewReader(buf))
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
