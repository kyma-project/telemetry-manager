package trace

import (
	"fmt"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

func HaveFlatTraces(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonTraces []byte) ([]FlatTrace, error) {
		tds, err := unmarshalTraces(jsonTraces)
		if err != nil {
			return nil, fmt.Errorf("HaveFlatTraces requires a valid OTLP JSON document: %w", err)
		}

		ft := flattenAllTraces(tds)
		return ft, nil
	}, matcher)

}

// HaveName extracts name from FlatTrace and applies the matcher to it.
func HaveName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(ft FlatTrace) string {
		return ft.Name
	}, matcher)
}

// HaveResourceAttributes extracts resource attributes from FlatTrace and applies the matcher to them.
func HaveResourceAttributes(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(ft FlatTrace) map[string]string {
		return ft.ResourceAttributes
	}, matcher)
}

// HaveSpanAttributes extracts span attributes from FlatTrace and applies the matcher to them.
func HaveSpanAttributes(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(ft FlatTrace) map[string]string {
		return ft.SpanAttributes
	}, matcher)
}
