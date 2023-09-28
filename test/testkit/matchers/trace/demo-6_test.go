package trace

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Demo-6", func() {
	It("has spans with color refactored (failing)", func() {
		Expect(jsonlTraces).Should(ContainTd(ContainSpan(WithSpanAttrs(HaveKeyWithValue("color", BeElementOf("yellow", "green"))))))
	})
})
