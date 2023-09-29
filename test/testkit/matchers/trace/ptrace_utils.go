package trace

import (
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/kyma-project/telemetry-manager/test/testkit/matchers"
)

func unmarshalTraces(jsonlMetrics []byte) ([]ptrace.Traces, error) {
	return matchers.UnmarshalSignals[ptrace.Traces](jsonlMetrics, func(buf []byte) (ptrace.Traces, error) {
		var unmarshaler ptrace.JSONUnmarshaler
		return unmarshaler.UnmarshalTraces(buf)
	})
}

func getSpans(td ptrace.Traces) []ptrace.Span {
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
}
