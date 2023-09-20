package matchers

import (
	"fmt"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// ConsistOfSpansWithIDs succeeds if the filexporter output file consists of spans only with the span ids passed into the matcher. The ordering of the elements does not matter.
func ConsistOfSpansWithIDs(expectedSpanIDs ...pcommon.SpanID) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlTraces []byte) ([]pcommon.SpanID, error) {
		tds, err := unmarshalTraces(jsonlTraces)
		if err != nil {
			return nil, fmt.Errorf("ConsistOfSpansWithIDs requires a valid OTLP JSON document: %v", err)
		}

		spans := getAllSpans(tds)

		var spanIDs []pcommon.SpanID
		for _, span := range spans {
			spanIDs = append(spanIDs, span.SpanID())
		}
		return spanIDs, nil
	}, gomega.ConsistOf(expectedSpanIDs))
}

// ConsistOfSpansWithTraceID succeeds if the filexporter output file only consists of spans the trace id passed into the matcher.
func ConsistOfSpansWithTraceID(expectedTraceID pcommon.TraceID) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlTraces []byte) ([]pcommon.TraceID, error) {
		tds, err := unmarshalTraces(jsonlTraces)
		if err != nil {
			return nil, fmt.Errorf("ConsistOfSpansWithTraceID requires a valid OTLP JSON document: %v", err)
		}

		spans := getAllSpans(tds)

		var traceIDs []pcommon.TraceID
		for _, span := range spans {
			traceIDs = append(traceIDs, span.TraceID())
		}
		return traceIDs, nil
	}, gomega.HaveEach(expectedTraceID))
}

// ConsistOfSpansWithAttributes succeeds if the filexporter output file consists of spans only with the span attributes passed into the matcher.
func ConsistOfSpansWithAttributes(expectedAttrs pcommon.Map) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlTraces []byte) ([]map[string]any, error) {
		tds, err := unmarshalTraces(jsonlTraces)
		if err != nil {
			return nil, fmt.Errorf("ConsistOfSpansWithAttributes requires a valid OTLP JSON document: %v", err)
		}

		spans := getAllSpans(tds)

		var attrs []map[string]any
		for _, span := range spans {
			attrs = append(attrs, span.Attributes().AsRaw())
		}
		return attrs, nil
	}, gomega.HaveEach(gomega.Equal(expectedAttrs.AsRaw())))
}

// ConsistOfNumberOfSpans succeeds if the filexporter output file has the expected number of spans.
func ConsistOfNumberOfSpans(expectedNumber int) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlTraces []byte) (int, error) {
		tds, err := unmarshalTraces(jsonlTraces)
		if err != nil {
			return 0, fmt.Errorf("ConsistOfNumberOfSpans requires a valid OTLP JSON document: %v", err)
		}

		spans := getAllSpans(tds)
		return len(spans), nil
	}, gomega.Equal(expectedNumber))
}

// ContainSpansWithAttributes succeeds if the filexporter output file has one or more spans with the span attributes passed into the matcher.
func ContainSpansWithAttributes(expectedAttrs pcommon.Map) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlTraces []byte) ([]ptrace.Span, error) {
		tds, err := unmarshalTraces(jsonlTraces)
		if err != nil {
			return nil, fmt.Errorf("ContainSpansWithAttributes requires a valid OTLP JSON document: %v", err)
		}

		spans := getAllSpans(tds)

		var matchingSpans []ptrace.Span
		for _, span := range spans {
			if hasMatchingAttributes(span.Attributes(), expectedAttrs) {
				matchingSpans = append(matchingSpans, span)
			}
		}
		return matchingSpans, nil
	}, gomega.Not(gomega.BeEmpty()))
}

func ContainSpansWithResourceAttributes(expectedAttrs pcommon.Map) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlTraces []byte) ([]ptrace.Span, error) {
		tds, err := unmarshalTraces(jsonlTraces)
		if err != nil {
			return nil, fmt.Errorf("ContainSpansWithResourceAttributes requires a valid OTLP JSON document: %v", err)
		}

		matchingSpans := getSpansWithResourceAttributes(tds, expectedAttrs)
		return matchingSpans, nil
	}, gomega.Not(gomega.BeEmpty()))
}

func hasMatchingAttributes(actualAttrs pcommon.Map, expectedAttrs pcommon.Map) bool {
	for expectedKey, expectedVal := range expectedAttrs.AsRaw() {
		if value, hasKey := actualAttrs.Get(expectedKey); !hasKey || value.AsString() != expectedVal {
			return false
		}
	}
	return true
}

func getSpansWithResourceAttributes(tds []ptrace.Traces, expectedAttrs pcommon.Map) []ptrace.Span {
	var spans []ptrace.Span

	for _, td := range tds {
		for i := 0; i < td.ResourceSpans().Len(); i++ {
			resourceSpans := td.ResourceSpans().At(i)
			if !hasMatchingAttributes(resourceSpans.Resource().Attributes(), expectedAttrs) {
				continue
			}
			for j := 0; j < resourceSpans.ScopeSpans().Len(); j++ {
				scopeSpans := resourceSpans.ScopeSpans().At(j)
				for k := 0; k < scopeSpans.Spans().Len(); k++ {
					spans = append(spans, scopeSpans.Spans().At(k))
				}
			}
		}
	}

	return spans
}

func getAllSpans(tds []ptrace.Traces) []ptrace.Span {
	var spans []ptrace.Span

	for _, td := range tds {
		for i := 0; i < td.ResourceSpans().Len(); i++ {
			resourceSpans := td.ResourceSpans().At(i)
			for j := 0; j < resourceSpans.ScopeSpans().Len(); j++ {
				scopeSpans := resourceSpans.ScopeSpans().At(j)
				for k := 0; k < scopeSpans.Spans().Len(); k++ {
					spans = append(spans, scopeSpans.Spans().At(k))
				}
			}
		}
	}

	return spans
}

func unmarshalTraces(jsonlTraces []byte) ([]ptrace.Traces, error) {
	return UnmarshalSignals[ptrace.Traces](jsonlTraces, func(buf []byte) (ptrace.Traces, error) {
		var unmarshaler ptrace.JSONUnmarshaler
		return unmarshaler.UnmarshalTraces(buf)
	})
}
