//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
)

type TraceData struct {
	ResourceSpans []ResourceSpans `json:"resourceSpans"`
}

type ResourceSpans struct {
	Resource   Resource     `json:"resource"`
	ScopeSpans []ScopeSpans `json:"scopeSpans"`
}

type Resource struct {
	Attributes []Attributes `json:"attributes"`
}

type Attributes struct {
	Key   string `json:"key"`
	Value Value  `json:"value"`
}

type Value struct {
	StringValue string `json:"stringValue"`
}

type ScopeSpans struct {
	Spans []Span `json:"spans"`
}

type Span struct {
	TraceID           string       `json:"traceId"`
	SpanID            string       `json:"spanId"`
	ParentSpanID      string       `json:"parentSpanId"`
	Name              string       `json:"name"`
	Kind              int          `json:"kind"`
	StartTimeUnixNano string       `json:"startTimeUnixNano"`
	EndTimeUnixNano   string       `json:"endTimeUnixNano"`
	Attributes        []Attributes `json:"attributes"`
}

func getSpans(traceDataJSON []byte, traceID string) ([]Span, error) {
	var traceData TraceData
	if err := json.Unmarshal(traceDataJSON, &traceData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal trace data json: %v", err)
	}

	var spans []Span
	for _, resourceSpans := range traceData.ResourceSpans {
		for _, scopeSpans := range resourceSpans.ScopeSpans {
			for _, span := range scopeSpans.Spans {
				if span.TraceID == traceID {
					spans = append(spans, span)
				}
			}
		}
	}

	return spans, nil
}
