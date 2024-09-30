package trace

import (
	"encoding/hex"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var fts = []FlatTrace{
	{
		Name:         "ingress",
		TraceID:      "92192b91842ba1873d247b7dbc6766b7",
		SpanID:       "3d5417cb19489c26",
		ScopeName:    "container",
		ScopeVersion: "1.0",
		ResourceAttributes: map[string]string{
			"service.name":        "backend",
			"k8s.pod.ip":          "10.42.1.76",
			"k8s.deployment.name": "backend",
		},
		ScopeAttributes: map[string]string{},
		SpanAttributes: map[string]string{
			"response_size":           "31",
			"upstream_cluster.name":   "inbound|4317||",
			"istio.canonical_service": "backend",
		},
	},
	{
		Name:         "ingress-2",
		TraceID:      "92192b91842ba1873d247b7dbc6766b7",
		SpanID:       "3d5417cb19489c26",
		ScopeName:    "runtime",
		ScopeVersion: "2.0",
		ResourceAttributes: map[string]string{
			"service.name":        "monitoring-custom-metrics",
			"k8s.pod.ip":          "10.42.1.73",
			"k8s.deployment.name": "istio",
		},
		ScopeAttributes: map[string]string{"foo": "bar"},
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
		scope.Scope().Attributes()

		//set span name, traceID, spanID
		s := scope.Spans().AppendEmpty()
		s.SetName("ingress")

		//set trace and span ID
		tID, err := newTraceIDWithInput("92192b91842ba1873d247b7dbc6766b7")
		Expect(err).ToNot(HaveOccurred())
		sID, err := newSpanIDWithInput("3d5417cb19489c26")
		Expect(err).ToNot(HaveOccurred())
		s.SetTraceID(tID)
		s.SetSpanID(sID)

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

var _ = Describe("HaveSpanID", func() {
	It("should apply matcher", func() {
		Expect(fts).Should(ContainElement(HaveSpanID(Equal("3d5417cb19489c26"))))
	})
})

var _ = Describe("HaveTraceID", func() {
	It("should apply matcher", func() {
		Expect(fts).Should(ContainElement(HaveTraceID(Equal("92192b91842ba1873d247b7dbc6766b7"))))
	})
})

var _ = Describe("HaveScopeName", func() {
	It("should apply matcher", func() {
		Expect(fts).Should(ContainElement(HaveScopeName(Equal("runtime"))))
	})
})

var _ = Describe("HaveScopeVersion", func() {
	It("should apply matcher", func() {
		Expect(fts).Should(ContainElement(HaveScopeVersion(Equal("2.0"))))
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

//var _ = Describe("WithSpanIDs", func() {
//	It("should apply matcher", func() {
//		td := ptrace.NewTraces()
//		rs := td.ResourceSpans().AppendEmpty()
//		spans := rs.ScopeSpans().AppendEmpty().Spans()
//
//		spanIDs := []pcommon.SpanID{newSpanID(), newSpanID()}
//		spans.AppendEmpty().SetSpanID(spanIDs[0])
//		spans.AppendEmpty().SetSpanID(spanIDs[1])
//
//		Expect(mustMarshalTraces(td)).Should(ContainTd(WithSpans(WithSpanIDs(ConsistOf(spanIDs)))))
//	})
//})

func mustMarshalTraces(td ptrace.Traces) []byte {
	var marshaler ptrace.JSONMarshaler
	bytes, err := marshaler.MarshalTraces(td)
	if err != nil {
		panic(err)
	}
	return bytes
}

func newTraceIDWithInput(s string) (pcommon.TraceID, error) {
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return pcommon.TraceID{}, fmt.Errorf("error while decoding string: %w", err)
	}
	if len(decoded) != 16 {
		return pcommon.TraceID{}, fmt.Errorf("traceID length does not match 16 bytes")
	}
	tID := pcommon.TraceID{}
	copy(tID[:], decoded)
	return tID, nil
}
func newSpanIDWithInput(s string) (pcommon.SpanID, error) {
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return pcommon.SpanID{}, fmt.Errorf("error while decoding string: %w", err)
	}
	if len(decoded) != 8 {
		return pcommon.SpanID{}, fmt.Errorf("spanID length does not match 8 bytes")
	}
	sID := pcommon.SpanID{}
	copy(sID[:], decoded)
	return sID, nil
}
