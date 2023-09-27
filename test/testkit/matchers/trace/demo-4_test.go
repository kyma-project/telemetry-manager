package trace

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var _ = Describe("Demo-4", func() {
	It("has spans with color", func() {
		Expect(jsonlTraces).Should(ContainTd(ContainSpan(WithTransform(func(span ptrace.Span) map[string]any {
			return span.Attributes().AsRaw()
		}, HaveKey("color")))))
	})
})
