package matchers

import (
	"bytes"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/prometheus/common/expfmt"
)

func ContainValidPrometheusMetric(metricName string) types.GomegaMatcher {
	return gomega.WithTransform(func(responseBodyBytes []byte) (bool, error) {
		var parser expfmt.TextParser
		mf, err := parser.TextToMetricFamilies(bytes.NewReader(responseBodyBytes))

		if err != nil {
			// ignore duplicate metrics parsing error and try extract metric
			return mf[metricName] != nil, nil
		}
		return mf[metricName] != nil, nil
	}, gomega.BeTrue())
}
