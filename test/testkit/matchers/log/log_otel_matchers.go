package log

import (
	"fmt"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

func HaveFlatOtelLogs(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonLogs []byte) ([]FlatLogOtel, error) {
		tds, err := unmarshalOtelLogs(jsonLogs)
		if err != nil {
			return nil, fmt.Errorf("HaveFlatOtelLogs requires a valid OTLP JSON document: %w", err)
		}

		ft := flattenAllOtelLogs(tds)

		return ft, nil
	}, matcher)
}

// HaveResourceAttributes extracts resource attributes from FlatLog and applies the matcher to them.
func HaveResourceAttributes(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogOtel) map[string]string {
		return fl.ResourceAttributes
	}, matcher)
}
