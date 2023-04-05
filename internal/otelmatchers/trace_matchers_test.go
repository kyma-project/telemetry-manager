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

	Context("with no matching span IDs", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/consist_of_spans_with_ids_no_match.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail", func() {
			Expect(fileBytes).ShouldNot(ConsistOfSpansWithIDs(expectedSpansWithIDs))
		})
	})

	Context("with partially matching span IDs", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/consist_of_spans_with_ids_partial_match.jsonl")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail", func() {
			Expect(fileBytes).ShouldNot(ConsistOfSpansWithIDs(expectedSpansWithIDs))
		})
	})

	Context("with fully matching span IDs", func() {
		BeforeEach(func() {
			var err error
			fileBytes, err = os.ReadFile("testdata/consist_of_spans_with_ids_full_match.jsonl")
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
