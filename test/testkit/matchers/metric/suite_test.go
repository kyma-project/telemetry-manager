package metric

import (
	"testing"

	"github.com/onsi/gomega"

	. "github.com/onsi/ginkgo/v2"
)

func TestMetricMatchers(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "Metric Matcher Suite")
}
