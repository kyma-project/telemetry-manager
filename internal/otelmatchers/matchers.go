package otelmatchers

import (
	"fmt"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"go.opentelemetry.io/otel/attribute"
)

func ConsistOfSpansWithIDs(expectedSpanIDs []string) types.GomegaMatcher {
	return gomega.WithTransform(func(actual interface{}) ([]string, error) {
		actualBytes, ok := actual.([]byte)
		if !ok {
			return nil, fmt.Errorf("ConsistOfSpansWithIDs rqquires a []byte, but got %T", actual)
		}

		actualSpans, err := unmarshalOTLPJSONTraceData(actualBytes)
		if err != nil {
			return nil, fmt.Errorf("ConsistOfSpansWithIDs requires a valid OTLP JSON document: %v", err)
		}

		var actualSpanIDs []string
		for _, span := range actualSpans {
			actualSpanIDs = append(actualSpanIDs, span.SpanID)
		}
		return actualSpanIDs, nil
	}, gomega.ConsistOf(expectedSpanIDs))
}

func EachHaveTraceID(expectedTraceID string) types.GomegaMatcher {
	return gomega.WithTransform(func(actual interface{}) ([]string, error) {
		actualBytes, ok := actual.([]byte)
		if !ok {
			return nil, fmt.Errorf("EachHaveTraceID rqquires a []byte, but got %T", actual)
		}

		actualSpans, err := unmarshalOTLPJSONTraceData(actualBytes)
		if err != nil {
			return nil, fmt.Errorf("EachHaveTraceID requires a valid OTLP JSON document: %v", err)
		}

		var actualTraceIDs []string
		for _, span := range actualSpans {
			actualTraceIDs = append(actualTraceIDs, span.TraceID)
		}
		return actualTraceIDs, nil
	}, gomega.HaveEach(expectedTraceID))
}

func EachHaveAttributes(expectedAttrs []attribute.KeyValue) types.GomegaMatcher {
	return gomega.WithTransform(func(actual interface{}) ([]map[string]string, error) {
		actualBytes, ok := actual.([]byte)
		if !ok {
			return nil, fmt.Errorf("EachHaveAttributes rqquires a []byte, but got %T", actual)
		}

		actualSpans, err := unmarshalOTLPJSONTraceData(actualBytes)
		if err != nil {
			return nil, fmt.Errorf("EachHaveAttributes requires a valid OTLP JSON document: %v", err)
		}

		var actualAttrs []map[string]string
		for _, span := range actualSpans {
			actualAttrs = append(actualAttrs, spanAttributesToMap(span.Attributes))
		}
		return actualAttrs, nil
	}, gomega.HaveEach(gomega.Equal(attributesToMap(expectedAttrs))))
}

func spanAttributesToMap(attrs []Attribute) map[string]string {
	results := make(map[string]string)
	for _, attr := range attrs {
		results[attr.Key] = attr.Value.StringValue
	}
	return results
}

func attributesToMap(attrs []attribute.KeyValue) map[string]string {
	results := make(map[string]string)
	for _, attr := range attrs {
		results[string(attr.Key)] = attr.Value.AsString()
	}
	return results
}
