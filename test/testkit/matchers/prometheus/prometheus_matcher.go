package prometheus

import (
	"bytes"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/prometheus/common/expfmt"
)

func ContainPrometheusMetric(metricName string) types.GomegaMatcher {
	return gomega.WithTransform(func(responseBody []byte) (bool, error) {
		var parser expfmt.TextParser
		mf, err := parser.TextToMetricFamilies(bytes.NewReader(responseBody))

		if err != nil {
			// ignore duplicate metrics parsing error and try extract metric
			return mf[metricName] != nil, nil
		}
		return mf[metricName] != nil, nil
	}, gomega.BeTrue())
}
