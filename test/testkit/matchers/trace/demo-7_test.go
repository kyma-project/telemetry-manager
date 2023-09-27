package trace

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Demo-7", func() {
	It("ultimate matcher", func() {
		Expect(jsonlTraces).Should(ContainTd(
			SatisfyAll(
				ContainResourceAttrs(HaveKeyWithValue("k8s.cluster.name", "cluster-01")),
				ContainSpan(SatisfyAny(
					WithSpanID(ContainSubstring("foo")),
					WithSpanAttrs(HaveKey("color")),
				)),
			),
		))
	})
})
