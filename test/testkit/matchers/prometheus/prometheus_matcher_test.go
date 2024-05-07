package prometheus

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContainMetric", Label("metrics"), func() {
	Context("with nil input", func() {
		It("should fail", func() {
			success, err := ContainMetricFamily(WithName(Equal("foo_metric"))).Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should fail", func() {
			success, err := ContainMetricFamily(WithName(Equal("foo_metric"))).Match([]byte{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with invalid input", func() {
		It("should fail", func() {
			success, err := ContainMetricFamily(WithName(Equal("foo_metric"))).Match([]byte{1, 2, 3})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with having metrics", func() {
		It("should succeed", func() {
			fileBytes := `
# HELP fluentbit_uptime Number of seconds that Fluent Bit has been running.
# TYPE fluentbit_uptime counter
fluentbit_uptime{hostname="telemetry-fluent-bit-dglkf"} 5489
# HELP fluentbit_input_bytes_total Number of input bytes.
# TYPE fluentbit_input_bytes_total counter
fluentbit_input_bytes_total{name="tele-tail"} 5217998`
			Expect([]byte(fileBytes)).Should(ContainMetricFamily(WithName(Equal("fluentbit_uptime"))))
		})
	})
})

var _ = Describe("WithLabels", func() {
	It("should apply matcher", func() {
		fileBytes := `
# HELP fluentbit_uptime Number of seconds that Fluent Bit has been running.
# TYPE fluentbit_uptime counter
fluentbit_uptime{hostname="telemetry-fluent-bit-dglkf"} 5489
# HELP fluentbit_input_bytes_total Number of input bytes.
# TYPE fluentbit_input_bytes_total counter
fluentbit_input_bytes_total{name="tele-tail"} 5000
`
		Expect([]byte(fileBytes)).Should(ContainMetricFamily(SatisfyAll(
			WithName(Equal("fluentbit_input_bytes_total")),
			ContainMetric(WithLabels(HaveKeyWithValue("name", "tele-tail"))),
		)))
	})
})

var _ = Describe("WithValue", func() {
	It("should apply matcher", func() {
		fileBytes := `
# HELP fluentbit_uptime Number of seconds that Fluent Bit has been running.
# TYPE fluentbit_uptime counter
fluentbit_uptime{hostname="telemetry-fluent-bit-dglkf"} 5489
# HELP fluentbit_input_bytes_total Number of input bytes.
# TYPE fluentbit_input_bytes_total counter
fluentbit_input_bytes_total{name="tele-tail"} 5000
`
		Expect([]byte(fileBytes)).Should(ContainMetricFamily(SatisfyAll(
			WithName(Equal("fluentbit_input_bytes_total")),
			ContainMetric(SatisfyAll(
				WithLabels(HaveKeyWithValue("name", "tele-tail")),
				WithValue(BeNumerically(">=", 0)),
			)),
		)))
	})
})
