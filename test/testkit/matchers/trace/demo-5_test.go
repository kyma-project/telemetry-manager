package trace

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Demo-5", func() {
	It("has spans with color refactored", func() {
		Expect(jsonlTraces).Should(ContainTd(ContainSpan(WithSpanAttrs(HaveKey("color")))))
		Expect(jsonlTraces).Should(ContainTd(ContainSpan(WithSpanAttrs(HaveKeyWithValue("color", "red")))))
		Expect(jsonlTraces).Should(ContainTd(ContainSpan(WithSpanAttrs(HaveKeyWithValue("color", BeElementOf("red", "blue"))))))
	})
})
