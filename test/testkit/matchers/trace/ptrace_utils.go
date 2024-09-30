package trace

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/kyma-project/telemetry-manager/test/testkit/matchers"
)

type FlatTrace struct {
	Name, TraceID, SpanID, ScopeName, ScopeVersion      string
	ResourceAttributes, ScopeAttributes, SpanAttributes map[string]string
}

func unmarshalTraces(jsonlMetrics []byte) ([]ptrace.Traces, error) {
	return matchers.UnmarshalSignals[ptrace.Traces](jsonlMetrics, func(buf []byte) (ptrace.Traces, error) {
		var unmarshaler ptrace.JSONUnmarshaler
		return unmarshaler.UnmarshalTraces(buf)
	})
}

func flattenAllTraces(tds []ptrace.Traces) []FlatTrace {
	var flatTraces []FlatTrace

	for _, td := range tds {
		flatTraces = append(flatTraces, flattenTraces(td)...)
	}
	return flatTraces
}

func flattenTraces(td ptrace.Traces) []FlatTrace {
	var flatTraces []FlatTrace

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		resourceSpans := td.ResourceSpans().At(i)
		for j := 0; j < resourceSpans.ScopeSpans().Len(); j++ {
			scopeSpans := resourceSpans.ScopeSpans().At(j)
			for k := 0; k < scopeSpans.Spans().Len(); k++ {
				spans := scopeSpans.Spans().At(k)
				for l := 0; l < spans.Attributes().Len(); l++ {
					flatTraces = append(flatTraces, FlatTrace{
						Name:               spans.Name(),
						TraceID:            spans.TraceID().String(),
						SpanID:             spans.SpanID().String(),
						ScopeName:          scopeSpans.Scope().Name(),
						ScopeVersion:       scopeSpans.Scope().Version(),
						ResourceAttributes: attributeToMap(resourceSpans.Resource().Attributes()),
						ScopeAttributes:    attributeToMap(scopeSpans.Scope().Attributes()),
						SpanAttributes:     attributeToMap(spans.Attributes()),
					})
				}
			}
		}
	}
	return flatTraces
}

// attributeToMap converts pdata.AttributeMap to a map using the string representation of the values.
func attributeToMap(attrs pcommon.Map) map[string]string {
	attrMap := make(map[string]string)
	attrs.Range(func(k string, v pcommon.Value) bool {
		attrMap[k] = v.AsString()
		return true
	})
	return attrMap
}

//func getSpans(td ptrace.Traces) []ptrace.Span {
//	var spans []ptrace.Span
//
//	for i := 0; i < td.ResourceSpans().Len(); i++ {
//		resourceSpans := td.ResourceSpans().At(i)
//		for j := 0; j < resourceSpans.ScopeSpans().Len(); j++ {
//			scopeSpans := resourceSpans.ScopeSpans().At(j)
//			for k := 0; k < scopeSpans.Spans().Len(); k++ {
//				spans = append(spans, scopeSpans.Spans().At(k))
//			}
//		}
//	}
//
//	return spans
//}
