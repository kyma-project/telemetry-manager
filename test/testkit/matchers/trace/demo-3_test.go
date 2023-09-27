package trace

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Demo-3", func() {
	It("has 2 spans refactored", func() {
		Expect(jsonlTraces).Should(WithTds(ContainElements(WithSpans(HaveLen(2)))))

		// or even better
		Expect(jsonlTraces).Should(ContainTd(WithSpans(HaveLen(2))))
	})
})
