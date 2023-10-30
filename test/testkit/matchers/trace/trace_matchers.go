package trace

import (
	"fmt"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func WithTds(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlTraces []byte) ([]ptrace.Traces, error) {
		tds, err := unmarshalTraces(jsonlTraces)
		if err != nil {
			return nil, fmt.Errorf("WithTds requires a valid OTLP JSON document: %v", err)
		}

		return tds, nil
	}, matcher)
}

// ContainTd is an alias for WithTds(gomega.ContainElement()).
func ContainTd(matcher types.GomegaMatcher) types.GomegaMatcher {
	return WithTds(gomega.ContainElement(matcher))
}

// ConsistOfTds is an alias for WithTds(gomega.ConsistOf()).
func ConsistOfTds(matcher types.GomegaMatcher) types.GomegaMatcher {
	return WithTds(gomega.ConsistOf(matcher))
}

func WithResourceAttrs(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(td ptrace.Traces) ([]map[string]any, error) {
		var rawAttrs []map[string]any
		for i := 0; i < td.ResourceSpans().Len(); i++ {
			rawAttrs = append(rawAttrs, td.ResourceSpans().At(i).Resource().Attributes().AsRaw())
		}
		return rawAttrs, nil
	}, matcher)
}

// ContainResourceAttrs is an alias for WithResourceAttrs(gomega.ContainElement()).
func ContainResourceAttrs(matcher types.GomegaMatcher) types.GomegaMatcher {
	return WithResourceAttrs(gomega.ContainElement(matcher))
}

func WithSpans(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(td ptrace.Traces) ([]ptrace.Span, error) {
		return getSpans(td), nil
	}, matcher)
}

// ContainSpan is an alias for WithSpans(gomega.ContainElement()).
func ContainSpan(matcher types.GomegaMatcher) types.GomegaMatcher {
	return WithSpans(gomega.ContainElement(matcher))
}

// ConsistOfSpans is an alias for WithSpans(gomega.ConsistOf()).
func ConsistOfSpans(matcher types.GomegaMatcher) types.GomegaMatcher {
	return WithSpans(gomega.ConsistOf(matcher))
}

func WithTraceID(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(s ptrace.Span) pcommon.TraceID {
		return s.TraceID()
	}, matcher)
}

func WithSpanIDs(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(spans []ptrace.Span) []pcommon.SpanID {
		var spansIDs []pcommon.SpanID
		for _, span := range spans {
			spansIDs = append(spansIDs, span.SpanID())
		}
		return spansIDs
	}, matcher)
}

func WithSpanID(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(s ptrace.Span) pcommon.SpanID {
		return s.SpanID()
	}, matcher)
}

func WithSpanAttrs(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(s ptrace.Span) map[string]any {
		return s.Attributes().AsRaw()
	}, matcher)
}
