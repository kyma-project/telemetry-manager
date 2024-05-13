package metric

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

var _ = Describe("WithMds", func() {
	It("should apply matcher to valid metrics data", func() {
		md := pmetric.NewMetrics()
		Expect(mustMarshalMetrics(md)).Should(WithMds(ContainElements()))
	})

	It("should fail when given empty byte slice", func() {
		Expect([]byte{}).Should(WithMds(BeEmpty()))
	})

	It("should return error for nil input", func() {
		success, err := WithMds(BeEmpty()).Match(nil)
		Expect(err).Should(HaveOccurred())
		Expect(success).Should(BeFalse())
	})

	It("should return error for invalid input type", func() {
		success, err := WithMds(BeEmpty()).Match(struct{}{})
		Expect(err).Should(HaveOccurred())
		Expect(success).Should(BeFalse())
	})
})

var _ = Describe("WithResourceAttrs", func() {
	It("should apply matcher", func() {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		attrs := rm.Resource().Attributes()
		attrs.PutStr("k8s.cluster.name", "cluster-01")
		attrs.PutStr("k8s.deployment.name", "nginx")

		Expect(mustMarshalMetrics(md)).Should(ContainMd(WithResourceAttrs(ContainElement(HaveKey("k8s.cluster.name")))))
	})
})

var _ = Describe("WithMetrics", func() {
	It("should apply matcher", func() {
		md := pmetric.NewMetrics()
		md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()

		Expect(mustMarshalMetrics(md)).Should(ContainMd(WithMetrics(HaveLen(1))))
	})
})

var _ = Describe("WithName", func() {
	It("should apply matcher", func() {
		md := pmetric.NewMetrics()
		md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("container.cpu.time")

		Expect(mustMarshalMetrics(md)).Should(ContainMd(ContainMetric(WithName(ContainSubstring("container")))))
	})
})

var _ = Describe("WithType", func() {
	It("should apply matcher", func() {
		md := pmetric.NewMetrics()
		md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetEmptyGauge()

		Expect(mustMarshalMetrics(md)).Should(ContainMd(ContainMetric(WithType(Equal(pmetric.MetricTypeGauge)))))
	})
})

var _ = Describe("WithDataPointAttrs", func() {
	It("should apply matcher", func() {
		md := pmetric.NewMetrics()
		gauge := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetEmptyGauge()

		pts := gauge.DataPoints()

		pt := pts.AppendEmpty()
		pt.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		pt.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		pt.SetDoubleValue(1.5)
		pt.Attributes().PutStr("foo", "bar")

		Expect(mustMarshalMetrics(md)).Should(
			ContainMd(ContainMetric(WithDataPointAttrs(ContainElement(HaveKey("foo"))))),
		)
	})
})

var _ = Describe("Contain Instrumentation Scope", func() {
	It("should apply matcher", func() {
		md := pmetric.NewMetrics()
		md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Scope().SetName("container")

		Expect(mustMarshalMetrics(md)).Should(ContainMd(WithScope(HaveLen(1))))
		Expect(mustMarshalMetrics(md)).Should(ContainMd(WithScope(ContainElement(WithScopeName(ContainSubstring("container"))))))
	})
})

func mustMarshalMetrics(md pmetric.Metrics) []byte {
	var marshaler pmetric.JSONMarshaler
	bytes, err := marshaler.MarshalMetrics(md)
	if err != nil {
		panic(err)
	}
	return bytes
}
