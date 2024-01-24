package trace

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	kittraces "github.com/kyma-project/telemetry-manager/test/testkit/otel/traces"
)

var _ = Describe("WithTds", func() {
	It("should apply matcher to valid trace data", func() {
		td := ptrace.NewTraces()
		Expect(mustMarshalTraces(td)).Should(WithTds(ContainElements()))
	})

	It("should fail when given empty byte slice", func() {
		Expect([]byte{}).Should(WithTds(BeEmpty()))
	})

	It("should return error for nil input", func() {
		success, err := WithTds(BeEmpty()).Match(nil)
		Expect(err).Should(HaveOccurred())
		Expect(success).Should(BeFalse())
	})

	It("should return error for invalid input type", func() {
		success, err := WithTds(BeEmpty()).Match(struct{}{})
		Expect(err).Should(HaveOccurred())
		Expect(success).Should(BeFalse())
	})
})

var _ = Describe("WithResourceAttrs", func() {
	It("should apply matcher", func() {
		td := ptrace.NewTraces()
		rm := td.ResourceSpans().AppendEmpty()
		attrs := rm.Resource().Attributes()
		attrs.PutStr("k8s.cluster.name", "cluster-01")
		attrs.PutStr("k8s.deployment.name", "nginx")

		Expect(mustMarshalTraces(td)).Should(ContainTd(WithResourceAttrs(ContainElement(HaveKey("k8s.cluster.name")))))
	})
})

var _ = Describe("WithSpans", func() {
	It("should apply matcher", func() {
		td := ptrace.NewTraces()
		rs := td.ResourceSpans().AppendEmpty()
		spans := rs.ScopeSpans().AppendEmpty().Spans()
		spans.AppendEmpty()
		spans.AppendEmpty()

		Expect(mustMarshalTraces(td)).Should(ContainTd(WithSpans(HaveLen(2))))
	})
})

var _ = Describe("WithTraceID", func() {
	It("should apply matcher", func() {
		td := ptrace.NewTraces()
		rs := td.ResourceSpans().AppendEmpty()
		spans := rs.ScopeSpans().AppendEmpty().Spans()

		traceID := kittraces.NewTraceID()
		spans.AppendEmpty().SetTraceID(traceID)

		Expect(mustMarshalTraces(td)).Should(ContainTd(ContainSpan(WithTraceID(Equal(traceID)))))
	})
})

var _ = Describe("WithSpanID", func() {
	It("should apply matcher", func() {
		td := ptrace.NewTraces()
		rs := td.ResourceSpans().AppendEmpty()
		spans := rs.ScopeSpans().AppendEmpty().Spans()

		spanID := kittraces.NewSpanID()
		spans.AppendEmpty().SetSpanID(spanID)

		Expect(mustMarshalTraces(td)).Should(ContainTd(ContainSpan(WithSpanID(Equal(spanID)))))
	})
})

var _ = Describe("WithSpanIDs", func() {
	It("should apply matcher", func() {
		td := ptrace.NewTraces()
		rs := td.ResourceSpans().AppendEmpty()
		spans := rs.ScopeSpans().AppendEmpty().Spans()

		spanIDs := []pcommon.SpanID{kittraces.NewSpanID(), kittraces.NewSpanID()}
		spans.AppendEmpty().SetSpanID(spanIDs[0])
		spans.AppendEmpty().SetSpanID(spanIDs[1])

		Expect(mustMarshalTraces(td)).Should(ContainTd(WithSpans(WithSpanIDs(ConsistOf(spanIDs)))))
	})
})

var _ = Describe("WithSpanAttrs", func() {
	It("should apply matcher", func() {
		td := ptrace.NewTraces()
		rs := td.ResourceSpans().AppendEmpty()
		spans := rs.ScopeSpans().AppendEmpty().Spans()

		span := spans.AppendEmpty()
		attrs := span.Attributes()
		attrs.PutStr("color", "red")

		Expect(mustMarshalTraces(td)).Should(ContainTd(ContainSpan(WithSpanAttrs(HaveKey("color")))))
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
