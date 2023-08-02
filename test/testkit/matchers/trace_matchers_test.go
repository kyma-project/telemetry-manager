//go:build e2e

package matchers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kittraces "github.com/kyma-project/telemetry-manager/test/e2e/testkit/otlp/traces"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var _ = Describe("ConsistOfSpansWithIDs", Label("tracing"), func() {
	Context("with nil input", func() {
		It("should error", func() {
			success, err := ConsistOfSpansWithIDs(kittraces.NewSpanID()).Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with input of invalid type", func() {
		It("should error", func() {
			success, err := ConsistOfSpansWithIDs(kittraces.NewSpanID()).Match(struct{}{})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should fail", func() {
			Expect([]byte{}).ShouldNot(ConsistOfSpansWithIDs(kittraces.NewSpanID()))
		})
	})

	Context("with no spans containing the span IDs", func() {
		It("should fail", func() {
			td := ptrace.NewTraces()
			spans := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
			spans.AppendEmpty().SetSpanID(kittraces.NewSpanID())
			spans.AppendEmpty().SetSpanID(kittraces.NewSpanID())
			spans.AppendEmpty().SetSpanID(kittraces.NewSpanID())

			Expect(mustMarshalTraces(td)).ShouldNot(ConsistOfSpansWithIDs(kittraces.NewSpanID(), kittraces.NewSpanID(), kittraces.NewSpanID()))
		})
	})

	Context("with some spans containing the span IDs", func() {
		It("should fail", func() {
			matchingSpanID := kittraces.NewSpanID()
			td := ptrace.NewTraces()
			spans := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
			spans.AppendEmpty().SetSpanID(matchingSpanID)
			spans.AppendEmpty().SetSpanID(kittraces.NewSpanID())
			spans.AppendEmpty().SetSpanID(kittraces.NewSpanID())

			nonMatchingSpanID := kittraces.NewSpanID()

			Expect(mustMarshalTraces(td)).ShouldNot(ConsistOfSpansWithIDs(matchingSpanID, nonMatchingSpanID))
		})
	})

	Context("with all spans containing only the span IDs", func() {
		It("should succeed", func() {
			matchingSpanIDs := []pcommon.SpanID{kittraces.NewSpanID(), kittraces.NewSpanID(), kittraces.NewSpanID()}
			td := ptrace.NewTraces()
			spans := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
			spans.AppendEmpty().SetSpanID(matchingSpanIDs[2])
			spans.AppendEmpty().SetSpanID(matchingSpanIDs[1])
			spans.AppendEmpty().SetSpanID(matchingSpanIDs[0])

			Expect(mustMarshalTraces(td)).Should(ConsistOfSpansWithIDs(matchingSpanIDs...))
		})
	})

	Context("with invalid input", func() {
		It("should error", func() {
			success, err := ConsistOfSpansWithIDs(kittraces.NewSpanID()).Match([]byte{1, 2, 3})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})
})

var _ = Describe("ConsistOfSpansWithTraceID", Label("tracing"), func() {
	Context("with nil input", func() {
		It("should error", func() {
			success, err := ConsistOfSpansWithTraceID(kittraces.NewTraceID()).Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with input of invalid type", func() {
		It("should error", func() {
			success, err := ConsistOfSpansWithTraceID(kittraces.NewTraceID()).Match(struct{}{})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should error", func() {
			success, err := ConsistOfSpansWithTraceID(kittraces.NewTraceID()).Match([]byte{})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with no spans having the trace ID", func() {
		It("should fail", func() {
			td := ptrace.NewTraces()
			spans := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
			spans.AppendEmpty().SetTraceID(kittraces.NewTraceID())
			spans.AppendEmpty().SetTraceID(kittraces.NewTraceID())
			spans.AppendEmpty().SetTraceID(kittraces.NewTraceID())

			Expect(mustMarshalTraces(td)).ShouldNot(ConsistOfSpansWithTraceID(kittraces.NewTraceID()))
		})
	})

	Context("with some spans having the trace ID", func() {
		It("should fail", func() {
			matchingTraceID := kittraces.NewTraceID()
			td := ptrace.NewTraces()
			spans := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
			spans.AppendEmpty().SetTraceID(matchingTraceID)
			spans.AppendEmpty().SetTraceID(kittraces.NewTraceID())
			spans.AppendEmpty().SetTraceID(kittraces.NewTraceID())

			Expect(mustMarshalTraces(td)).ShouldNot(ConsistOfSpansWithTraceID(matchingTraceID))
		})
	})

	Context("with all spans having the trace ID", func() {
		It("should succeed", func() {
			matchingTraceID := kittraces.NewTraceID()
			td := ptrace.NewTraces()
			spans := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
			spans.AppendEmpty().SetTraceID(matchingTraceID)
			spans.AppendEmpty().SetTraceID(matchingTraceID)
			spans.AppendEmpty().SetTraceID(matchingTraceID)

			Expect(mustMarshalTraces(td)).Should(ConsistOfSpansWithTraceID(matchingTraceID))
		})
	})

	Context("with invalid input", func() {
		It("should error", func() {
			success, err := ConsistOfSpansWithTraceID(kittraces.NewTraceID()).Match([]byte{1, 2, 3})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})
})

var _ = Describe("ConsistOfSpansWithAttributes", Label("tracing"), func() {
	Context("with nil input", func() {
		It("should error", func() {
			success, err := ConsistOfSpansWithAttributes(pcommon.NewMap()).Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with input of invalid type", func() {
		It("should error", func() {
			success, err := ConsistOfSpansWithAttributes(pcommon.NewMap()).Match(struct{}{})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should error", func() {
			success, err := ConsistOfSpansWithAttributes(pcommon.NewMap()).Match([]byte{})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with no spans having the attributes", func() {
		It("should fail", func() {
			td := ptrace.NewTraces()
			spans := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
			spans.AppendEmpty().Attributes().PutStr("http.url", "foo")
			spans.AppendEmpty().Attributes().PutStr("http.url", "bar")
			spans.AppendEmpty().Attributes().PutStr("http.url", "baz")

			expectedAttrs := pcommon.NewMap()
			expectedAttrs.PutStr("http.method", "GET")
			Expect(mustMarshalTraces(td)).ShouldNot(ConsistOfSpansWithAttributes(expectedAttrs))
		})
	})

	Context("with some spans having the attributes", func() {
		It("should fail", func() {
			td := ptrace.NewTraces()
			spans := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
			spans.AppendEmpty().Attributes().PutStr("http.url", "foo")
			spans.AppendEmpty().Attributes().PutStr("http.url", "bar")
			spans.AppendEmpty().Attributes().PutStr("http.url", "baz")

			expectedAttrs := pcommon.NewMap()
			expectedAttrs.PutStr("http.url", "foo")
			Expect(mustMarshalTraces(td)).ShouldNot(ConsistOfSpansWithAttributes(expectedAttrs))
		})
	})

	Context("with all spans having only the expected attributes", func() {
		It("should succeed", func() {
			td := ptrace.NewTraces()
			spans := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
			spans.AppendEmpty().Attributes().PutStr("http.url", "foo")
			spans.AppendEmpty().Attributes().PutStr("http.url", "foo")
			spans.AppendEmpty().Attributes().PutStr("http.url", "foo")

			expectedAttrs := pcommon.NewMap()
			expectedAttrs.PutStr("http.url", "foo")
			Expect(mustMarshalTraces(td)).Should(ConsistOfSpansWithAttributes(expectedAttrs))
		})
	})

	Context("with all spans containing the expected and some other attributes", func() {
		It("should succeed", func() {
			td := ptrace.NewTraces()
			spans := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
			attrs := spans.AppendEmpty().Attributes()
			attrs.PutStr("http.url", "foo")
			attrs.PutStr("http.method", "GET")
			spans.AppendEmpty().Attributes().PutStr("http.url", "foo")
			spans.AppendEmpty().Attributes().PutStr("http.url", "foo")

			expectedAttrs := pcommon.NewMap()
			expectedAttrs.PutStr("http.url", "foo")
			Expect(mustMarshalTraces(td)).ShouldNot(ConsistOfSpansWithAttributes(expectedAttrs))
		})
	})

	Context("with invalid input", func() {
		It("should error", func() {
			success, err := ConsistOfSpansWithAttributes(pcommon.NewMap()).Match([]byte{1, 2, 3})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})
})

var _ = Describe("ConsistOfNumberOfSpans", Label("tracing"), func() {
	Context("with nil input", func() {
		It("should match 0", func() {
			success, err := ConsistOfNumberOfSpans(0).Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should match 0", func() {
			success, err := ConsistOfNumberOfSpans(0).Match([]byte{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(success).Should(BeTrue())
		})
	})

	Context("with invalid input", func() {
		It("should error", func() {
			success, err := ConsistOfNumberOfSpans(0).Match([]byte{1, 2, 3})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with not matching number of spans", func() {
		It("should succeed", func() {
			td := ptrace.NewTraces()
			spans := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
			spans.AppendEmpty()
			spans.AppendEmpty()

			Expect(mustMarshalTraces(td)).ShouldNot(ConsistOfNumberOfSpans(3))
		})
	})

	Context("with matching number of spans", func() {
		It("should succeed", func() {
			td := ptrace.NewTraces()
			spans := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
			spans.AppendEmpty()
			spans.AppendEmpty()

			Expect(mustMarshalTraces(td)).Should(ConsistOfNumberOfSpans(2))
		})
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
