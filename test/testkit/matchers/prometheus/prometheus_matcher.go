package prometheus

import (
	"bytes"

	"github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

func HaveFlatMetricFamilies(matcher gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return gomega.WithTransform(func(responseBody []byte) ([]FlatMetricFamily, error) {
		parser := expfmt.NewTextParser(model.UTF8Validation)

		mfs, _ := parser.TextToMetricFamilies(bytes.NewReader(responseBody)) //nolint:errcheck // ignore duplicate metrics parsing error and try extract metric

		fmfs := flattenAllMetricFamilies(mfs)

		return fmfs, nil
	}, matcher)
}

func HaveName(matcher gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return gomega.WithTransform(func(fmf FlatMetricFamily) string {
		return fmf.Name
	}, matcher)
}

func HaveMetricValue(matcher gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return gomega.WithTransform(func(fmf FlatMetricFamily) float64 {
		return fmf.MetricValue
	}, matcher)
}

func HaveLabels(matcher gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return gomega.WithTransform(func(fmf FlatMetricFamily) map[string]string {
		return fmf.Labels
	}, matcher)
}
