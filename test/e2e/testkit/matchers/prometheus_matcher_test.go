//go:build e2e

package matchers

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HasValidPrometheusMetric", func() {
	var fileBytes []byte

	Context("with nil input", func() {
		It("should fail", func() {
			success, err := HasValidPrometheusMetric("foo_metric").Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should fail", func() {
			success, err := HasValidPrometheusMetric("foo_metric").Match([]byte{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with invalid input", func() {
		BeforeEach(func() {
			fileBytes = []byte{1, 2, 3}
		})

		It("should fail", func() {
			success, err := HasValidPrometheusMetric("foo_metric").Match(fileBytes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with having metrics", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/prometheus/metrics.txt")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed", func() {
			Expect(fileBytes).Should(HasValidPrometheusMetric("fluentbit_uptime"))
		})
	})
})
