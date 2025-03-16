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

// HaveScopeName extracts scope name from FlatLog and applies the matcher to it.
func HaveScopeName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogOtel) string {
		return fl.ScopeName
	}, matcher)
}

// HaveScopeVersion extracts scope version from FlatLog and applies the matcher to it.
func HaveScopeVersion(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogOtel) string {
		return fl.ScopeVersion
	}, matcher)
}

// HaveAttributes extracts resource attributes from FlatLog and applies the matcher to them.
func HaveAttributes(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogOtel) map[string]string {
		return fl.Attributes
	}, matcher)
}

func HaveLogRecordBody(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogOtel) string { return fl.LogRecordBody }, matcher)
}

func HaveObservedTimestamp(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogOtel) string { return fl.ObservedTimestamp }, matcher)
}

func HaveOtelTimestamp(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogOtel) string { return fl.Timestamp }, matcher)
}
