package metric

import (
	"go.opentelemetry.io/collector/pdata/pmetric"

	"fmt"
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers"
	kitmetrics "github.com/kyma-project/telemetry-manager/test/testkit/otlp/metrics"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

func ContainMd(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlMetrics []byte) ([]pmetric.Metrics, error) {
		mds, err := extractMetrics(jsonlMetrics)
		if err != nil {
			return nil, fmt.Errorf("ContainMd requires a valid OTLP JSON document: %v", err)
		}

		return mds, nil
	}, gomega.ContainElements(matcher))
}

func ConsistOfMds(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlMetrics []byte) ([]pmetric.Metrics, error) {
		mds, err := extractMetrics(jsonlMetrics)
		if err != nil {
			return nil, fmt.Errorf("ConsistOfMds requires a valid OTLP JSON document: %v", err)
		}

		return mds, nil
	}, gomega.ConsistOf(matcher))
}

func WithMetrics(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(md pmetric.Metrics) ([]pmetric.Metric, error) {
		return kitmetrics.AllMetrics(md), nil
	}, matcher)
}

func WithResourceAttrs(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(md pmetric.Metrics) ([]map[string]any, error) {
		var rawAttrs []map[string]any
		for i := 0; i < md.ResourceMetrics().Len(); i++ {
			rawAttrs = append(rawAttrs, md.ResourceMetrics().At(i).Resource().Attributes().AsRaw())
		}
		return rawAttrs, nil
	}, matcher)
}

func WithName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(m pmetric.Metric) (string, error) {
		return m.Name(), nil
	}, matcher)
}

func WithType(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(m pmetric.Metric) (pmetric.MetricType, error) {
		return m.Type(), nil
	}, matcher)
}

func WithDataPointAttrs(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(m pmetric.Metric) ([]map[string]any, error) {
		var rawAttrs []map[string]any
		for _, attrs := range kitmetrics.GetAttributesPerDataPoint(m) {
			rawAttrs = append(rawAttrs, attrs.AsRaw())
		}
		return rawAttrs, nil
	}, matcher)
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
	return matchers.UnmarshalSignals[pmetric.Metrics](jsonlMetrics, func(buf []byte) (pmetric.Metrics, error) {
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
