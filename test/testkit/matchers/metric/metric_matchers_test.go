package metric

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

var fmdps = []FlatMetric{
	{
		Name:               "container.cpu.time",
		Description:        "time of container cpu",
		ScopeName:          "runtime",
		ScopeVersion:       "1.0",
		ScopeAttributes:    map[string]string{"baz": "qux"},
		ResourceAttributes: map[string]string{"k8s.cluster.name": "cluster-01", "k8s.deployment.name": "nginx"},
		MetricAttributes:   map[string]string{"foo": "bar"},
		Type:               "Gauge",
	},
	{
		Name:               "container.cpu.usage",
		Description:        "usage of container cpu",
		ScopeName:          "container",
		ScopeVersion:       "2.0",
		ScopeAttributes:    map[string]string{"bar": "baz"},
		ResourceAttributes: map[string]string{"k8s.cluster.name": "cluster-01", "k8s.deployment.name": "istio"},
		MetricAttributes:   map[string]string{"metricAttr": "valueMetricAttr"},
		Type:               "Gauge",
	},
}

var _ = Describe("HaveFlatMetrics", func() {
	It("should apply matcher to valid metrics data", func() {
		md := pmetric.NewMetrics()
		Expect(mustMarshalMetrics(md)).Should(HaveFlatMetrics(ContainElements()))
	})

	It("should fail when given empty byte slice", func() {
		Expect([]byte{}).Should(HaveFlatMetrics(BeEmpty()))
	})

	It("should return error for nil input", func() {
		success, err := HaveFlatMetrics(BeEmpty()).Match(nil)
		Expect(err).Should(HaveOccurred())
		Expect(success).Should(BeFalse())
	})

	It("should return error for invalid input type", func() {
		success, err := HaveFlatMetrics(BeEmpty()).Match(struct{}{})
		Expect(err).Should(HaveOccurred())
		Expect(success).Should(BeFalse())
	})

	It("should return a FlatMetricDataPoints structure", func() {
		md := pmetric.NewMetrics()

		rm := md.ResourceMetrics().AppendEmpty()
		attrs := rm.Resource().Attributes()
		attrs.PutStr("k8s.cluster.name", "cluster-01")
		attrs.PutStr("k8s.deployment.name", "nginx")

		s := rm.ScopeMetrics().AppendEmpty()

		s.Scope().SetName("runtime")
		s.Scope().SetVersion("1.0")
		s.Scope().Attributes().PutStr("baz", "qux")

		m := s.Metrics().AppendEmpty()
		m.SetName("container.cpu.time")
		m.SetDescription("time of container cpu")
		gauge := m.SetEmptyGauge()
		pt := gauge.DataPoints().AppendEmpty()

		pt.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		pt.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		pt.SetDoubleValue(1.5)
		pt.Attributes().PutStr("foo", "bar")

		Expect(mustMarshalMetrics(md)).Should(HaveFlatMetrics(ContainElement(fmdps[0])))
	})
})

var _ = Describe("HaveUniqueNames", func() {
	It("should return unique names", func() {
		Expect(fmdps).Should(HaveUniqueNames(ConsistOf("container.cpu.time", "container.cpu.usage")))
	})
})

var _ = Describe("HaveResourceAttributes", func() {
	It("should have the specified key", func() {
		Expect(fmdps).Should(ContainElement(HaveResourceAttributes(HaveKey("k8s.cluster.name"))))
	})
})

var _ = Describe("HaveName", func() {
	It("should return the correct name", func() {
		Expect(fmdps).Should(ContainElement(HaveName(ContainSubstring("container"))))
	})
})

var _ = Describe("HaveType", func() {
	It("should return the correct type", func() {
		Expect(fmdps).Should(ContainElement(HaveType(Equal(pmetric.MetricTypeGauge.String()))))
	})
})

var _ = Describe("HaveMetricAttributes", func() {
	It("should have the specified key", func() {
		Expect(fmdps).Should(
			ContainElement(HaveMetricAttributes(HaveKey("foo"))),
		)
	})
})

var _ = Describe("HaveScopeName", func() {
	It("should contain the specified string", func() {
		Expect(fmdps).Should(ContainElement(HaveScopeName(ContainSubstring("container"))))
	})
})

var _ = Describe("HaveScopeVersion", func() {
	It("should contain the specified version", func() {
		Expect(fmdps).Should(ContainElement(HaveScopeVersion(ContainSubstring("1.0"))))
	})
})

var _ = Describe("HaveKeys", func() {
	It("should have all the keys within the specified list", func() {
		Expect(fmdps).Should(ContainElement(HaveResourceAttributes(HaveKeys(ContainElements("k8s.cluster.name", "k8s.deployment.name")))))
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
