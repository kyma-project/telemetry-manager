package trace

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/kyma-project/telemetry-manager/test/testkit/matchers"
)

// FlatTrace holds all needed information about a trace.
// Gomega doesn't handle deeply nested data structure very well and generates large, unreadable diffs when paired with the deeply nested structure of pmetrics.
//
// Introducing a go struct with a flat data structure by extracting necessary information from different levels of ptraces makes accessing the information easier than using ptrace.Traces directly and improves the readability of the test output logs.
type FlatTrace struct {
	Name                               string
	ResourceAttributes, SpanAttributes map[string]string
}

func unmarshalTraces(jsonlMetrics []byte) ([]ptrace.Traces, error) {
	return matchers.UnmarshalSignals[ptrace.Traces](jsonlMetrics, func(buf []byte) (ptrace.Traces, error) {
		var unmarshaler ptrace.JSONUnmarshaler
		return unmarshaler.UnmarshalTraces(buf)
	})
}

// flattenAllTraces flattens an array of ptrace.Traces datapoints to a slice of FlatTrace.
// It converts the deeply nested ptrace.Traces data structure to a flat struct, to make it more readable in the test output logs.
func flattenAllTraces(tds []ptrace.Traces) []FlatTrace {
	var flatTraces []FlatTrace

	for _, td := range tds {
		flatTraces = append(flatTraces, flattenTraces(td)...)
	}
	return flatTraces
}

// flattenTraces converts a single ptrace.Traces datapoint to a slice of FlatTrace
// It takes relevant information from different levels of pdata and puts it into a FlatTrace go struct.
func flattenTraces(td ptrace.Traces) []FlatTrace {
	var flatTraces []FlatTrace

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		resourceSpans := td.ResourceSpans().At(i)
		for j := 0; j < resourceSpans.ScopeSpans().Len(); j++ {
			scopeSpans := resourceSpans.ScopeSpans().At(j)
			for k := 0; k < scopeSpans.Spans().Len(); k++ {
				span := scopeSpans.Spans().At(k)
				flatTraces = append(flatTraces, FlatTrace{
					Name:               span.Name(),
					ResourceAttributes: attributeToMap(resourceSpans.Resource().Attributes()),
					SpanAttributes:     attributeToMap(span.Attributes()),
				})
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
