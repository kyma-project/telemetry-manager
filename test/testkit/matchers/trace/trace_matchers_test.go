package trace

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var fts = []FlatTrace{
	{
		Name: "ingress",
		ResourceAttributes: map[string]string{
			"service.name":        "backend",
			"k8s.pod.ip":          "10.42.1.76",
			"k8s.deployment.name": "backend",
		},
		SpanAttributes: map[string]string{
			"response_size":           "31",
			"upstream_cluster.name":   "inbound|4317||",
			"istio.canonical_service": "backend",
		},
	},
	{
		Name: "ingress-2",
		ResourceAttributes: map[string]string{
			"service.name":        "monitoring-custom-metrics",
			"k8s.pod.ip":          "10.42.1.73",
			"k8s.deployment.name": "istio",
		},
		SpanAttributes: map[string]string{
			"response_size":           "32",
			"upstream_cluster.name":   "inbound|4318||",
			"istio.canonical_service": "istio",
		},
	},
}

var _ = Describe("HaveFlatTraces", func() {
	It("should apply matcher to valid trace data", func() {
		td := ptrace.NewTraces()
		Expect(mustMarshalTraces(td)).Should(HaveFlatTraces(ContainElements()))
	})

	It("should fail when given empty byte slice", func() {
		Expect([]byte{}).Should(HaveFlatTraces(BeEmpty()))
	})

	It("should return error for nil input", func() {
		success, err := HaveFlatTraces(BeEmpty()).Match(nil)
		Expect(err).Should(HaveOccurred())
		Expect(success).Should(BeFalse())
	})

	It("should return error for invalid input type", func() {
		success, err := HaveFlatTraces(BeEmpty()).Match(struct{}{})
		Expect(err).Should(HaveOccurred())
		Expect(success).Should(BeFalse())
	})

	It("should return a FlatTrace struct", func() {
		td := ptrace.NewTraces()
		//set resource attributes
		rt := td.ResourceSpans().AppendEmpty()
		attrs := rt.Resource().Attributes()
		attrs.PutStr("service.name", "backend")
		attrs.PutStr("k8s.pod.ip", "10.42.1.76")
		attrs.PutStr("k8s.deployment.name", "backend")
		//set scope version, name and attributes
		scope := rt.ScopeSpans().AppendEmpty()
		scope.Scope().SetName("container")
		scope.Scope().SetVersion("1.0")

		//set span name, traceID, spanID
		s := scope.Spans().AppendEmpty()
		s.SetName("ingress")

		//set span attributes
		s.Attributes().PutStr("response_size", "31")
		s.Attributes().PutStr("upstream_cluster.name", "inbound|4317||")
		s.Attributes().PutStr("istio.canonical_service", "backend")

		Expect(mustMarshalTraces(td)).Should(HaveFlatTraces(ContainElements(fts[0])))
	})
})

var _ = Describe("HaveName", func() {
	It("should apply matcher", func() {
		Expect(fts).Should(ContainElement(HaveName(Equal("ingress"))))
	})
})

var _ = Describe("HaveResourceAttributes", func() {
	It("should apply matcher", func() {

		Expect(fts).Should(ContainElement(HaveResourceAttributes(HaveKey("k8s.deployment.name"))))
	})
})

var _ = Describe("HaveSpanAttributes", func() {
	It("should apply matcher", func() {
		Expect(fts).Should(ContainElement(HaveSpanAttributes(HaveKey("response_size"))))
	})
})

func mustMarshalTraces(td ptrace.Traces) []byte {
	var marshaler ptrace.JSONMarshaler
	bytes, err := marshaler.MarshalTraces(td)
	if err != nil {
		panic(err)
	}
	return bytes
}
