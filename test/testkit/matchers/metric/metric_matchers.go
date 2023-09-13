package metric

import (
	"fmt"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"go.opentelemetry.io/collector/pdata/pmetric"
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
		return getMetrics(md), nil
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
		for _, attrs := range getAttributesPerDataPoint(m) {
			rawAttrs = append(rawAttrs, attrs.AsRaw())
		}
		return rawAttrs, nil
	}, matcher)
}
