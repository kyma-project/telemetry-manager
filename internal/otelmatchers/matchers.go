package otelmatchers

import (
	"fmt"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

func HaveSpanIDs(spanIDs []string) types.GomegaMatcher {
	return gomega.WithTransform(func(actual interface{}) ([]string, error) {
		actualBytes, ok := actual.([]byte)
		if !ok {
			return nil, fmt.Errorf("HaveSpanIDs expects a []byte, but got %T", actual)
		}

		spans, err := parseFileExporterTraceOutput(actualBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse span data %v", err)
		}

		var spanIDs []string
		for _, span := range spans {
			spanIDs = append(spanIDs, span.SpanID)
		}

		return spanIDs, nil
	}, gomega.ContainElements(spanIDs))
}
