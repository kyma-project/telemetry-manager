package otelmatchers

import (
	"os"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCustomMatcher(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Custom Matcher Suite")
}

var _ = Describe("ConsistOfSpansWithIDs", func() {
	var fileBytes []byte
	var expectedSpansWithIDs []pcommon.SpanID

	BeforeEach(func() {
		expectedSpansWithIDs = []pcommon.SpanID{[8]byte{1}, [8]byte{2}, [8]byte{3}}
	})

	Context("with nil input", func() {
		BeforeEach(func() {
			fileBytes = nil
		})

		It("should fail", func() {
			Expect(fileBytes).ShouldNot(ConsistOfSpansWithIDs(expectedSpansWithIDs))
		})
	})

	Context("with input of invalid type", func() {
		It("should error", func() {
			_, err := ConsistOfSpansWithIDs(expectedSpansWithIDs).Match(struct{}{})
			Expect(err).Should(HaveOccurred())
		})
	})

	Context("with no spans matching the span IDs", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/consist_of_spans_with_ids/no_match.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail", func() {
			Expect(fileBytes).ShouldNot(ConsistOfSpansWithIDs(expectedSpansWithIDs))
		})
	})

	Context("with some spans matching the span IDs", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/consist_of_spans_with_ids/partial_match.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail", func() {
			Expect(fileBytes).ShouldNot(ConsistOfSpansWithIDs(expectedSpansWithIDs))
		})
	})

	Context("with all spans matching the span IDs", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/consist_of_spans_with_ids/full_match.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed", func() {
			Expect(fileBytes).Should(ConsistOfSpansWithIDs(expectedSpansWithIDs))
		})
	})

	Context("with invalid input", func() {
		BeforeEach(func() {
			fileBytes = []byte{1, 2, 3}
		})

		It("should error", func() {
			_, err := ConsistOfSpansWithIDs(expectedSpansWithIDs).Match(fileBytes)
			Expect(err).Should(HaveOccurred())
		})
	})
})

var _ = Describe("ConsistOfSpansWithTraceID", func() {
	var fileBytes []byte
	var expectedTraceID pcommon.TraceID

	BeforeEach(func() {
		expectedTraceID = [16]byte{1}
	})

	Context("with nil input", func() {
		BeforeEach(func() {
			fileBytes = nil
		})

		It("should error", func() {
			_, err := ConsistOfSpansWithTraceID(expectedTraceID).Match(fileBytes)
			Expect(err).Should(HaveOccurred())
		})
	})

	Context("with input of invalid type", func() {
		It("should error", func() {
			_, err := ConsistOfSpansWithTraceID(expectedTraceID).Match(struct{}{})
			Expect(err).Should(HaveOccurred())
		})
	})

	Context("with no spans having the trace ID", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/consist_of_spans_with_trace_id/no_match.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail", func() {
			Expect(fileBytes).ShouldNot(ConsistOfSpansWithTraceID(expectedTraceID))
		})
	})

	Context("with some spans having the trace ID", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/consist_of_spans_with_trace_id/partial_match.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail", func() {
			Expect(fileBytes).ShouldNot(ConsistOfSpansWithTraceID(expectedTraceID))
		})
	})

	Context("with all spans having the trace ID", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/consist_of_spans_with_trace_id/full_match.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed", func() {
			Expect(fileBytes).Should(ConsistOfSpansWithTraceID(expectedTraceID))
		})
	})

	Context("with invalid input", func() {
		BeforeEach(func() {
			fileBytes = []byte{1, 2, 3}
		})

		It("should error", func() {
			_, err := ConsistOfSpansWithTraceID(expectedTraceID).Match(fileBytes)
			Expect(err).Should(HaveOccurred())
		})
	})
})

var _ = Describe("ConsistOfSpansWithAttributes", func() {
	var fileBytes []byte
	var expectedAttrs pcommon.Map

	BeforeEach(func() {
		expectedAttrs = pcommon.NewMap()
		expectedAttrs.PutStr("strKey", "strValue")
		expectedAttrs.PutInt("intKey", 1)
		expectedAttrs.PutBool("boolKey", true)
	})

	Context("with nil input", func() {
		BeforeEach(func() {
			fileBytes = nil
		})

		It("should error", func() {
			_, err := ConsistOfSpansWithAttributes(expectedAttrs).Match(fileBytes)
			Expect(err).Should(HaveOccurred())
		})
	})

	Context("with input of invalid type", func() {
		It("should error", func() {
			_, err := ConsistOfSpansWithAttributes(expectedAttrs).Match(struct{}{})
			Expect(err).Should(HaveOccurred())
		})
	})

	Context("with no spans having the attributes", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/consist_of_spans_with_attributes/no_match.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail", func() {
			Expect(fileBytes).ShouldNot(ConsistOfSpansWithAttributes(expectedAttrs))
		})
	})

	Context("with some spans having the attributes", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/consist_of_spans_with_attributes/partial_match.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail", func() {
			Expect(fileBytes).ShouldNot(ConsistOfSpansWithAttributes(expectedAttrs))
		})
	})

	Context("with all spans having the attributes", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/consist_of_spans_with_attributes/full_match.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should succeed", func() {
			Expect(fileBytes).Should(ConsistOfSpansWithAttributes(expectedAttrs))
		})
	})

	Context("with invalid input", func() {
		BeforeEach(func() {
			fileBytes = []byte{1, 2, 3}
		})

		It("should error", func() {
			_, err := ConsistOfSpansWithAttributes(expectedAttrs).Match(fileBytes)
			Expect(err).Should(HaveOccurred())
		})
	})
})
