package prometheus

import (
	"bytes"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

func WithMetricFamilies(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(responseBody []byte) ([]*dto.MetricFamily, error) {
		var parser expfmt.TextParser
		// ignore duplicate metrics parsing error and try extract metric
		mfs, _ := parser.TextToMetricFamilies(bytes.NewReader(responseBody))
		var result []*dto.MetricFamily
		for _, mf := range mfs {
			result = append(result, mf)
		}
		return result, nil
	}, matcher)
}

func ContainMetricFamily(matcher types.GomegaMatcher) types.GomegaMatcher {
	return WithMetricFamilies(gomega.ContainElement(matcher))
}

func WithName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(mf *dto.MetricFamily) string {
		return mf.GetName()
	}, matcher)
}

func WithMetrics(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(mf *dto.MetricFamily) []*dto.Metric {
		return mf.GetMetric()
	}, matcher)
}

func ContainMetric(matcher types.GomegaMatcher) types.GomegaMatcher {
	return WithMetrics(gomega.ContainElement(matcher))
}

func WithValue(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(m *dto.Metric) (float64, error) {
		if m.Gauge != nil {
			return m.Gauge.GetValue(), nil
		}
		if m.Counter != nil {
			return m.Counter.GetValue(), nil
		}
		if m.Untyped != nil {
			return m.Untyped.GetValue(), nil
		}
		return 0, nil
	}, matcher)
}

func WithLabels(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(m *dto.Metric) (map[string]string, error) {
		labels := make(map[string]string)
		for _, l := range m.Label {
			labels[l.GetName()] = l.GetValue()
		}
		return labels, nil
	}, matcher)
}
