package log

import (
	"fmt"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

func HaveFlatOTLPLogs(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonLogs []byte) ([]FlatLogOTLP, error) {
		tds, err := unmarshalOTLPLogs(jsonLogs)
		if err != nil {
			return nil, fmt.Errorf("HaveFlatOTLPLogs requires a valid OTLP JSON document: %w", err)
		}

		ft := flattenAllOTLPLogs(tds)

		return ft, nil
	}, matcher)
}

// HaveResourceAttributes extracts resource attributes from FlatLog and applies the matcher to them.
func HaveResourceAttributes(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogOTLP) map[string]string {
		return fl.ResourceAttributes
	}, matcher)
}
