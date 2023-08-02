//go:build e2e

package matchers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/telemetry-manager/test/e2e/testkit/otlp/traces"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var _ = Describe("ConsistOfSpansWithIDs", Label("tracing"), func() {
	Context("with nil input", func() {
		It("should error", func() {
			success, err := ConsistOfSpansWithIDs(traces.NewSpanID()).Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with input of invalid type", func() {
		It("should error", func() {
			success, err := ConsistOfSpansWithIDs(traces.NewSpanID()).Match(struct{}{})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should fail", func() {
			Expect([]byte{}).ShouldNot(ConsistOfSpansWithIDs(traces.NewSpanID()))
		})
	})

	Context("with no spans containing the span IDs", func() {
		It("should fail", func() {
			td := ptrace.NewTraces()
			spans := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
			spans.AppendEmpty().SetSpanID(traces.NewSpanID())
			spans.AppendEmpty().SetSpanID(traces.NewSpanID())
			spans.AppendEmpty().SetSpanID(traces.NewSpanID())

			Expect(mustMarshalTraces(td)).ShouldNot(ConsistOfSpansWithIDs(traces.NewSpanID(), traces.NewSpanID(), traces.NewSpanID()))
		})
	})

	Context("with some spans containing the span IDs", func() {
		It("should fail", func() {
			matchingSpanID := traces.NewSpanID()
			td := ptrace.NewTraces()
			spans := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
			spans.AppendEmpty().SetSpanID(matchingSpanID)
			spans.AppendEmpty().SetSpanID(traces.NewSpanID())
			spans.AppendEmpty().SetSpanID(traces.NewSpanID())

			nonMatchingSpanID := traces.NewSpanID()

			Expect(mustMarshalTraces(td)).ShouldNot(ConsistOfSpansWithIDs(matchingSpanID, nonMatchingSpanID))
		})
	})

	Context("with all spans containing only the span IDs", func() {
		It("should succeed", func() {
			matchingSpanIDs := []pcommon.SpanID{traces.NewSpanID(), traces.NewSpanID(), traces.NewSpanID()}
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
			success, err := ConsistOfSpansWithIDs(traces.NewSpanID()).Match([]byte{1, 2, 3})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})
})

var _ = Describe("ConsistOfSpansWithTraceID", Label("tracing"), func() {
	Context("with nil input", func() {
		It("should error", func() {
			success, err := ConsistOfSpansWithTraceID(traces.NewTraceID()).Match(nil)
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with input of invalid type", func() {
		It("should error", func() {
			success, err := ConsistOfSpansWithTraceID(traces.NewTraceID()).Match(struct{}{})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with empty input", func() {
		It("should error", func() {
			success, err := ConsistOfSpansWithTraceID(traces.NewTraceID()).Match([]byte{})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})

	Context("with no spans having the trace ID", func() {
		It("should fail", func() {
			td := ptrace.NewTraces()
			spans := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
			spans.AppendEmpty().SetTraceID(traces.NewTraceID())
			spans.AppendEmpty().SetTraceID(traces.NewTraceID())
			spans.AppendEmpty().SetTraceID(traces.NewTraceID())

			Expect(mustMarshalTraces(td)).ShouldNot(ConsistOfSpansWithTraceID(traces.NewTraceID()))
		})
	})

	Context("with some spans having the trace ID", func() {
		It("should fail", func() {
			matchingTraceID := traces.NewTraceID()
			td := ptrace.NewTraces()
			spans := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
			spans.AppendEmpty().SetTraceID(matchingTraceID)
			spans.AppendEmpty().SetTraceID(traces.NewTraceID())
			spans.AppendEmpty().SetTraceID(traces.NewTraceID())

			Expect(mustMarshalTraces(td)).ShouldNot(ConsistOfSpansWithTraceID(matchingTraceID))
		})
	})

	Context("with all spans having the trace ID", func() {
		It("should succeed", func() {
			matchingTraceID := traces.NewTraceID()
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
			success, err := ConsistOfSpansWithTraceID(traces.NewTraceID()).Match([]byte{1, 2, 3})
			Expect(err).Should(HaveOccurred())
			Expect(success).Should(BeFalse())
		})
	})
})

//var _ = Describe("ConsistOfSpansWithAttributes", Label("tracing"), func() {
//	var fileBytes []byte
//	var expectedAttrs pcommon.Map
//
//	BeforeEach(func() {
//		expectedAttrs = pcommon.NewMap()
//		expectedAttrs.PutStr("strKey", "strValue")
//		expectedAttrs.PutInt("intKey", 1)
//		expectedAttrs.PutBool("boolKey", true)
//	})
//
//	Context("with nil input", func() {
//		It("should error", func() {
//			success, err := ConsistOfSpansWithAttributes(expectedAttrs).Match(fileBytes)
//			Expect(err).Should(HaveOccurred())
//			Expect(success).Should(BeFalse())
//		})
//	})
//
//	Context("with input of invalid type", func() {
//		It("should error", func() {
//			success, err := ConsistOfSpansWithAttributes(expectedAttrs).Match(struct{}{})
//			Expect(err).Should(HaveOccurred())
//			Expect(success).Should(BeFalse())
//		})
//	})
//
//	Context("with empty input", func() {
//		It("should error", func() {
//			success, err := ConsistOfSpansWithAttributes(expectedAttrs).Match([]byte{})
//			Expect(err).Should(HaveOccurred())
//			Expect(success).Should(BeFalse())
//		})
//	})
//
//	Context("with no spans having the attributes", func() {
//		BeforeEach(func() {
//			var err error
//			fileBytes, err = os.ReadFile("testdata/consist_of_spans_with_attributes/no_match.jsonl")
//			Expect(err).NotTo(HaveOccurred())
//		})
//
//		It("should fail", func() {
//			Expect(fileBytes).ShouldNot(ConsistOfSpansWithAttributes(expectedAttrs))
//		})
//	})
//
//	Context("with some spans having the attributes", func() {
//		BeforeEach(func() {
//			var err error
//			fileBytes, err = os.ReadFile("testdata/consist_of_spans_with_attributes/partial_match.jsonl")
//			Expect(err).NotTo(HaveOccurred())
//		})
//
//		It("should fail", func() {
//			Expect(fileBytes).ShouldNot(ConsistOfSpansWithAttributes(expectedAttrs))
//		})
//	})
//
//	Context("with all spans having the attributes", func() {
//		BeforeEach(func() {
//			var err error
//			fileBytes, err = os.ReadFile("testdata/consist_of_spans_with_attributes/full_match.jsonl")
//			Expect(err).NotTo(HaveOccurred())
//		})
//
//		It("should succeed", func() {
//			Expect(fileBytes).Should(ConsistOfSpansWithAttributes(expectedAttrs))
//		})
//	})
//
//	Context("with invalid input", func() {
//		BeforeEach(func() {
//			fileBytes = []byte{1, 2, 3}
//		})
//
//		It("should error", func() {
//			success, err := ConsistOfSpansWithAttributes(expectedAttrs).Match(fileBytes)
//			Expect(err).Should(HaveOccurred())
//			Expect(success).Should(BeFalse())
//		})
//	})
//})

//var _ = Describe("ConsistOfNumberOfSpans", Label("tracing"), func() {
//	var fileBytes []byte
//
//	Context("with nil input", func() {
//		It("should match 0", func() {
//			success, err := ConsistOfNumberOfSpans(0).Match(nil)
//			Expect(err).ShouldNot(HaveOccurred())
//			Expect(success).Should(BeTrue())
//		})
//	})
//
//	Context("with empty input", func() {
//		It("should match 0", func() {
//			success, err := ConsistOfNumberOfSpans(0).Match([]byte{})
//			Expect(err).ShouldNot(HaveOccurred())
//			Expect(success).Should(BeTrue())
//		})
//	})
//
//	Context("with invalid input", func() {
//		BeforeEach(func() {
//			fileBytes = []byte{1, 2, 3}
//		})
//
//		It("should error", func() {
//			success, err := ConsistOfNumberOfSpans(0).Match(fileBytes)
//			Expect(err).Should(HaveOccurred())
//			Expect(success).Should(BeFalse())
//		})
//	})
//
//	Context("with having spans", func() {
//		BeforeEach(func() {
//			var err error
//			fileBytes, err = os.ReadFile("testdata/consist_of_spans_with_attributes/full_match.jsonl")
//			Expect(err).NotTo(HaveOccurred())
//		})
//
//		It("should succeed", func() {
//			Expect(fileBytes).Should(ConsistOfNumberOfSpans(3))
//		})
//	})
//
//})

func mustMarshalTraces(td ptrace.Traces) []byte {
	var marshaler ptrace.JSONMarshaler
	bytes, err := marshaler.MarshalTraces(td)
	if err != nil {
		panic(err)
	}
	return bytes
}
