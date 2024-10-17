package prometheus

import (
	"bytes"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/prometheus/common/expfmt"
)

func HaveFlatMetricFamilies(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(responseBody []byte) ([]FlatMetricFamily, error) {
		var parser expfmt.TextParser
		mfs, _ := parser.TextToMetricFamilies(bytes.NewReader(responseBody)) //nolint:errcheck // ignore duplicate metrics parsing error and try extract metric

		fmfs := flattenAllMetricFamily(mfs)

		return fmfs, nil
	}, matcher)
}

func HaveName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fmf FlatMetricFamily) string {
		return fmf.Name
	}, matcher)
}

func HaveMetricValue(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fmf FlatMetricFamily) float64 {
		return fmf.MetricValues
	}, matcher)
}

func HaveLabels(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fmf FlatMetricFamily) map[string]string {
		return fmf.Labels
	}, matcher)
}
