package trace

import (
	"go.opentelemetry.io/collector/pdata/ptrace"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Demo-2", func() {
	It("has 2 spans", func() {
		Expect(jsonlTraces).Should(WithTransform(func(jsonlTraces []byte) ([]ptrace.Traces, error) {
			tds, err := unmarshalTraces(jsonlTraces)
			if err != nil {
				return nil, err
			}

			return tds, nil
		}, ContainElements(WithTransform(func(td ptrace.Traces) []ptrace.Span {
			var spans []ptrace.Span

			for i := 0; i < td.ResourceSpans().Len(); i++ {
				resourceSpans := td.ResourceSpans().At(i)
				for j := 0; j < resourceSpans.ScopeSpans().Len(); j++ {
					scopeSpans := resourceSpans.ScopeSpans().At(j)
					for k := 0; k < scopeSpans.Spans().Len(); k++ {
						spans = append(spans, scopeSpans.Spans().At(k))
					}
				}
			}

			return spans
		}, HaveLen(2)))))
	})
})
