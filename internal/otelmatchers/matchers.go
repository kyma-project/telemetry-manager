package otelmatchers

import (
	"fmt"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"go.opentelemetry.io/otel/attribute"
)

func ConsistOfSpansWithIDs(spanIDs []string) types.GomegaMatcher {
	return gomega.WithTransform(func(actual interface{}) ([]string, error) {
		actualBytes, ok := actual.([]byte)
		if !ok {
			return nil, fmt.Errorf("ConsistOfSpansWithIDs rqquires a []byte, but got %T", actual)
		}

		spans, err := unmarshalOTLPJSONTraceData(actualBytes)
		if err != nil {
			return nil, fmt.Errorf("ConsistOfSpansWithIDs requires a valid OTLP JSON document: %v", err)
		}

		var spanIDs []string
		for _, span := range spans {
			spanIDs = append(spanIDs, span.SpanID)
		}
		return spanIDs, nil
	}, gomega.ConsistOf(spanIDs))
}

func ConsistOfSpansWithAttributes(attrs []attribute.KeyValue) types.GomegaMatcher {
	return gomega.WithTransform(func(actual interface{}) ([]map[string]string, error) {
		actualBytes, ok := actual.([]byte)
		if !ok {
			return nil, fmt.Errorf("ConsistOfSpansWithAttributes rqquires a []byte, but got %T", actual)
		}

		spans, err := unmarshalOTLPJSONTraceData(actualBytes)
		if err != nil {
			return nil, fmt.Errorf("ConsistOfSpansWithAttributes requires a valid OTLP JSON document: %v", err)
		}

		var attrsAsMap []map[string]string
		for _, span := range spans {
			attrsAsMap = append(attrsAsMap, spanAttributesToMap(span.Attributes))
		}
		return attrsAsMap, nil
	}, gomega.HaveEach(gomega.Equal(attributesToMap(attrs))))
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
