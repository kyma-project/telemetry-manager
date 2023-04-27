//go:build e2e

package matchers

import (
	"os"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HaveGauges", func() {
	var fileBytes []byte
	var expectedGauges []pmetric.Gauge

	BeforeEach(func() {
		ts1 := pcommon.NewTimestampFromTime(time.Unix(0, 1682438376750990000))
		gauge1 := pmetric.NewGauge()
		dp11 := gauge1.DataPoints().AppendEmpty()
		dp11.SetTimestamp(ts1)
		dp11.SetStartTimestamp(ts1)
		dp11.SetDoubleValue(0.5)

		ts2 := pcommon.NewTimestampFromTime(time.Unix(0, 1682438376750991000))
		gauge2 := pmetric.NewGauge()
		dp21 := gauge2.DataPoints().AppendEmpty()
		dp21.SetTimestamp(ts2)
		dp21.SetStartTimestamp(ts2)
		dp21.SetDoubleValue(3.5)

		expectedGauges = []pmetric.Gauge{gauge1, gauge2}

		var marshaler pmetric.JSONMarshaler
		md := pmetric.NewMetrics()
		metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
		m1 := metrics.AppendEmpty()
		m1.SetName("room_temperature")
		gauge1.CopyTo(m1.SetEmptyGauge())

		m2 := metrics.AppendEmpty()
		m2.SetName("room_humidity")
		gauge2.CopyTo(m2.SetEmptyGauge())

		json, _ := marshaler.MarshalMetrics(md)
		os.WriteFile("testdata/have_gauges/full_match.jsonl", json, 0666)
	})

	Context("with nil input", func() {
		It("should error", func() {
			success, err := HaveGauges(expectedGauges...).Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with input of invalid type", func() {
		It("should error", func() {
			success, err := HaveGauges(expectedGauges...).Match(struct{}{})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should fail", func() {
			Expect([]byte{}).ShouldNot(HaveGauges(expectedGauges...))
		})
	})

	//Context("with no spans matching the span IDs", func() {
	//	BeforeEach(func() {
	//		var err error
	//		fileBytes, err = os.ReadFile("testdata/have_gauges/no_match.jsonl")
	//		Expect(err).NotTo(HaveOccurred())
	//	})
	//
	//	It("should fail", func() {
	//		Expect(fileBytes).ShouldNot(HaveGauges(expectedGauges...))
	//	})
	//})
	//
	//Context("with some spans matching the span IDs", func() {
	//	BeforeEach(func() {
	//		var err error
	//		fileBytes, err = os.ReadFile("testdata/have_gauges/partial_match.jsonl")
	//		Expect(err).NotTo(HaveOccurred())
	//	})
	//
	//	It("should fail", func() {
	//		Expect(fileBytes).ShouldNot(HaveGauges(expectedGauges...))
	//	})
	//})

	Context("with all spans matching the span IDs", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/have_gauges/full_match.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed", func() {
			Expect(fileBytes).Should(HaveGauges(expectedGauges...))
		})
	})

	Context("with invalid input", func() {
		BeforeEach(func() {
			fileBytes = []byte{1, 2, 3}
		})

		It("should error", func() {
			success, err := HaveGauges(expectedGauges...).Match(fileBytes)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})
})
