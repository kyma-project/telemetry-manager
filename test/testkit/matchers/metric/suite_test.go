package metric

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMetricMatchers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metric Matcher Suite")
}
