package otelmatchers

import (
	"bufio"
	"bytes"
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

func parseFileExporterTraceOutput(traceDataFile []byte) ([]Span, error) {
	var spans []Span

	scanner := bufio.NewScanner(bytes.NewReader(traceDataFile))
	for scanner.Scan() {
		var traceData TraceData
		if err := json.Unmarshal(scanner.Bytes(), &traceData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal trace data json: %v", err)
		}

		for _, resourceSpans := range traceData.ResourceSpans {
			for _, scopeSpans := range resourceSpans.ScopeSpans {
				spans = append(spans, scopeSpans.Spans...)
			}
		}

	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read trace data file: %v", err)
	}

	return spans, nil
}
