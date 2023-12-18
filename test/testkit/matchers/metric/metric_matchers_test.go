package metric

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/pmetric"

	kitmetrics "github.com/kyma-project/telemetry-manager/test/testkit/otel/metrics"
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
		metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
		gauge := kitmetrics.NewGauge()
		gauge.CopyTo(metrics.AppendEmpty())

		Expect(mustMarshalMetrics(md)).Should(ContainMd(WithMetrics(HaveLen(1))))
	})
})

var _ = Describe("WithName", func() {
	It("should apply matcher", func() {
		md := pmetric.NewMetrics()
		metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
		gauge := kitmetrics.NewGauge(kitmetrics.WithName("container.cpu.time"))
		gauge.CopyTo(metrics.AppendEmpty())

		Expect(mustMarshalMetrics(md)).Should(ContainMd(ContainMetric(WithName(ContainSubstring("container")))))
	})
})

var _ = Describe("WithType", func() {
	It("should apply matcher", func() {
		md := pmetric.NewMetrics()
		metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
		gauge := kitmetrics.NewGauge()
		gauge.CopyTo(metrics.AppendEmpty())

		Expect(mustMarshalMetrics(md)).Should(ContainMd(ContainMetric(WithType(Equal(pmetric.MetricTypeGauge)))))
	})
})

var _ = Describe("WithDataPointAttrs", func() {
	It("should apply matcher", func() {
		md := pmetric.NewMetrics()
		metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
		gauge := kitmetrics.NewGauge()
		gauge.CopyTo(metrics.AppendEmpty())

		//TODO: rewrite the test fixture builder to inject custom attrs
		Expect(mustMarshalMetrics(md)).Should(
			ContainMd(ContainMetric(WithDataPointAttrs(ContainElement(HaveKey("pt-label-key-0"))))),
		)
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
