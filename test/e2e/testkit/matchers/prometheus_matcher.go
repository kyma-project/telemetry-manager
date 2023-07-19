package matchers

import (
	"bytes"
	"fmt"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/prometheus/common/expfmt"
)

func HasValidPrometheusMetric(metricName string) types.GomegaMatcher {
	return gomega.WithTransform(func(actual interface{}) (bool, error) {
		actualResponse, ok := actual.([]byte)

		if !ok {
			return false, fmt.Errorf("HasValidPrometheusMetric requires a []byte, but got %T", actual)
		}

		var parser expfmt.TextParser
		mf, err := parser.TextToMetricFamilies(bytes.NewReader(actualResponse))

		if err != nil {
			// ignore duplicate metrics parsing error and try extract metric
			return mf[metricName] != nil, nil
		}
		return mf[metricName] != nil, nil
	}, gomega.BeTrue())
}
