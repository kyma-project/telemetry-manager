package prometheus

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestPrometheusMetricMatchers_VerifyInputs(t *testing.T) {
	RegisterTestingT(t)

	nilInput, err := HaveFlatMetricFamilies(ContainElement(HaveName(Equal("foo_metric")))).Match(nil)
	Expect(err).Should(HaveOccurred(), "Should return error for nil input")
	Expect(nilInput).Should(BeFalse(), "Success should be false for nil input")

	emptyInput, err := HaveFlatMetricFamilies(ContainElement(HaveName(Equal("foo_metric")))).Match([]byte{})
	Expect(err).ShouldNot(HaveOccurred(), "Should not return error for empty input")
	Expect(emptyInput).Should(BeFalse(), "Success should be false for empty input")

	invalidInput, err := HaveFlatMetricFamilies(ContainElement(HaveName(Equal("foo_metric")))).Match([]byte{1, 2, 3})
	Expect(err).ShouldNot(HaveOccurred(), "Should not return error for invalid input")
	Expect(invalidInput).Should(BeFalse(), "Success should be false for invalid input")
}

func TestPrometheusMetricMatchers(t *testing.T) {
	RegisterTestingT(t)

	fileBytesHaveName := `
# HELP fluentbit_uptime Number of seconds that Fluent Bit has been running.
# TYPE fluentbit_uptime counter
fluentbit_uptime{hostname="telemetry-fluent-bit-dglkf"} 5489
# HELP fluentbit_input_bytes_total Number of input bytes.
# TYPE fluentbit_input_bytes_total counter
fluentbit_input_bytes_total{name="tele-tail"} 5217998`
	Expect([]byte(fileBytesHaveName)).Should(HaveFlatMetricFamilies(ContainElement(HaveName(Equal("fluentbit_uptime")))), "should have metric with name fluentbit_uptime")

	fileBytesHaveLabels := `
# HELP fluentbit_uptime Number of seconds that Fluent Bit has been running.
# TYPE fluentbit_uptime counter
fluentbit_uptime{hostname="telemetry-fluent-bit-dglkf"} 5489
# HELP fluentbit_input_bytes_total Number of input bytes.
# TYPE fluentbit_input_bytes_total counter
fluentbit_input_bytes_total{name="tele-tail"} 5000
`
	Expect([]byte(fileBytesHaveLabels)).Should(HaveFlatMetricFamilies(ContainElement(SatisfyAll(
		HaveName(Equal("fluentbit_input_bytes_total")),
		HaveLabels(HaveKeyWithValue("name", "tele-tail")),
	))), "Should have metric with name fluentbit_input_bytes_total and label name=tele-tail")

	fileBytesHaveValue := `
# HELP fluentbit_uptime Number of seconds that Fluent Bit has been running.
# TYPE fluentbit_uptime counter
fluentbit_uptime{hostname="telemetry-fluent-bit-dglkf"} 5489
# HELP fluentbit_input_bytes_total Number of input bytes.
# TYPE fluentbit_input_bytes_total counter
fluentbit_input_bytes_total{name="tele-tail"} 5000
`
	Expect([]byte(fileBytesHaveValue)).Should(HaveFlatMetricFamilies(ContainElement(SatisfyAll(
		HaveName(Equal("fluentbit_input_bytes_total")),
		HaveLabels(HaveKeyWithValue("name", "tele-tail")),
		HaveMetricValue(BeNumerically(">=", 0)),
	))), "Should have metric with name fluentbit_input_bytes_total, label name=tele-tail and value >= 0")
}
