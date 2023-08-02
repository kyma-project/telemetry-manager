//go:build e2e

package matchers

import (
	"bufio"
	"bytes"
	"fmt"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// ConsistOfSpansWithIDs succeeds if the filexporter output file consists of spans only with the span ids passed into the matcher. The ordering of the elements does not matter.
func ConsistOfSpansWithIDs(expectedSpanIDs ...pcommon.SpanID) types.GomegaMatcher {
	return gomega.WithTransform(func(fileBytes []byte) ([]pcommon.SpanID, error) {
		actualTds, err := unmarshalOTLPJSONTraces(fileBytes)
		if err != nil {
			return nil, fmt.Errorf("ConsistOfSpansWithIDs requires a valid OTLP JSON document: %v", err)
		}

		actualSpans := getAllSpans(actualTds)

		var actualSpanIDs []pcommon.SpanID
		for _, span := range actualSpans {
			actualSpanIDs = append(actualSpanIDs, span.SpanID())
		}
		return actualSpanIDs, nil
	}, gomega.ConsistOf(expectedSpanIDs))
}

// ConsistOfSpansWithTraceID succeeds if the filexporter output file only consists of spans the trace id passed into the matcher.
func ConsistOfSpansWithTraceID(expectedTraceID pcommon.TraceID) types.GomegaMatcher {
	return gomega.WithTransform(func(fileBytes []byte) ([]pcommon.TraceID, error) {
		actualTds, err := unmarshalOTLPJSONTraces(fileBytes)
		if err != nil {
			return nil, fmt.Errorf("ConsistOfSpansWithTraceID requires a valid OTLP JSON document: %v", err)
		}

		actualSpans := getAllSpans(actualTds)

		var actualTraceIDs []pcommon.TraceID
		for _, span := range actualSpans {
			actualTraceIDs = append(actualTraceIDs, span.TraceID())
		}
		return actualTraceIDs, nil
	}, gomega.HaveEach(expectedTraceID))
}

// ConsistOfSpansWithAttributes succeeds if the filexporter output file consists of spans only with the span attributes passed into the matcher.
func ConsistOfSpansWithAttributes(expectedAttrs pcommon.Map) types.GomegaMatcher {
	return gomega.WithTransform(func(fileBytes []byte) ([]map[string]any, error) {
		actualTds, err := unmarshalOTLPJSONTraces(fileBytes)
		if err != nil {
			return nil, fmt.Errorf("ConsistOfSpansWithAttributes requires a valid OTLP JSON document: %v", err)
		}

		actualSpans := getAllSpans(actualTds)

		var actualAttrs []map[string]any
		for _, span := range actualSpans {
			actualAttrs = append(actualAttrs, span.Attributes().AsRaw())
		}
		return actualAttrs, nil
	}, gomega.HaveEach(gomega.Equal(expectedAttrs.AsRaw())))
}

// ConsistOfNumberOfSpans succeeds if the filexporter output file has the expected number of spans.
func ConsistOfNumberOfSpans(expectedNumber int) types.GomegaMatcher {
	return gomega.WithTransform(func(fileBytes []byte) (int, error) {
		actualTds, err := unmarshalOTLPJSONTraces(fileBytes)
		if err != nil {
			return 0, fmt.Errorf("ConsistOfNumberOfSpans requires a valid OTLP JSON document: %v", err)
		}

		actualSpans := getAllSpans(actualTds)
		return len(actualSpans), nil
	}, gomega.Equal(expectedNumber))
}

func getAllSpans(traces []ptrace.Traces) []ptrace.Span {
	var spans []ptrace.Span

	for _, td := range traces {
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

func unmarshalOTLPJSONTraces(buf []byte) ([]ptrace.Traces, error) {
	var results []ptrace.Traces

	var tracesUnmarshaler ptrace.JSONUnmarshaler
	scanner := bufio.NewScanner(bytes.NewReader(buf))
	for scanner.Scan() {
		td, err := tracesUnmarshaler.UnmarshalTraces(scanner.Bytes())
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshall traces: %v", err)
		}

		results = append(results, td)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read traces: %v", err)
	}

	return results, nil
}
