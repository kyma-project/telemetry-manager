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

func HaveGauges(expected []pmetric.Gauge) types.GomegaMatcher {
	return gomega.WithTransform(func(actual interface{}) ([]pmetric.Gauge, error) {
		actualBytes, ok := actual.([]byte)
		if !ok {
			return nil, fmt.Errorf("HaveGauges requires a []byte, but got %T", actual)
		}

		actualMetrics, err := unmarshalOTLPJSONMetrics(actualBytes)
		if err != nil {
			return nil, fmt.Errorf("HaveGauges requires a valid OTLP JSON document: %v", err)
		}

		var actualGauges []pmetric.Gauge
		for _, md := range actualMetrics {
			actualGauges = append(actualGauges, metrics.AllGauges(md)...)
		}

		return actualGauges, nil
	}, gomega.ContainElements(expected))
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
