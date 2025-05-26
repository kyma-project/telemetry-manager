package log

import (
	"fmt"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

func HaveFlatLogs(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonPLogs []byte) ([]FlatLog, error) {
		tds, err := unmarshalPLogs(jsonPLogs)
		if err != nil {
			return nil, fmt.Errorf("HaveFlatLogs requires a valid OTLP JSON document: %w", err)
		}

		ft := flattenAllLogs(tds)

		return ft, nil
	}, matcher)
}

// HaveResourceAttributes extracts resource attributes from FlatLog and applies the matcher to them.
func HaveResourceAttributes(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) map[string]string {
		return fl.ResourceAttributes
	}, matcher)
}

// HaveScopeName extracts scope name from FlatLog and applies the matcher to it.
func HaveScopeName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string {
		return fl.ScopeName
	}, matcher)
}

// HaveScopeVersion extracts scope version from FlatLog and applies the matcher to it.
func HaveScopeVersion(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string {
		return fl.ScopeVersion
	}, matcher)
}

// HaveAttributes extracts resource attributes from FlatLog and applies the matcher to them.
func HaveAttributes(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) map[string]string { return fl.Attributes }, matcher)
}

func HaveLogBody(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string { return fl.LogRecordBody }, matcher)
}

func HaveObservedTimestamp(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string { return fl.ObservedTimestamp }, matcher)
}

func HaveTimestamp(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string { return fl.Timestamp }, matcher)
}

func HaveTraceID(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string { return fl.TraceId }, matcher)
}

func HaveSpanID(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string { return fl.SpanId }, matcher)
}

func HaveTraceFlags(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) uint32 { return fl.TraceFlags }, matcher)
}

func HaveSeverityText(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string { return fl.SeverityText }, matcher)
}

func HaveSeverityNumber(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) int { return fl.SeverityNumber }, matcher)
}
