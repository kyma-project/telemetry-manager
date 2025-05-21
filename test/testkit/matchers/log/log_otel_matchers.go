package log

import (
	"fmt"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

func HaveFlatOTelLogs(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonLogs []byte) ([]FlatLogOTel, error) {
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
	return gomega.WithTransform(func(fl FlatLogOTel) map[string]string {
		return fl.ResourceAttributes
	}, matcher)
}

// HaveScopeName extracts scope name from FlatLog and applies the matcher to it.
func HaveScopeName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogOTel) string {
		return fl.ScopeName
	}, matcher)
}

// HaveScopeVersion extracts scope version from FlatLog and applies the matcher to it.
func HaveScopeVersion(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogOTel) string {
		return fl.ScopeVersion
	}, matcher)
}

// HaveAttributes extracts resource attributes from FlatLog and applies the matcher to them.
func HaveAttributes(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogOTel) map[string]string { return fl.Attributes }, matcher)
}

func HaveLogRecordBody(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogOTel) string { return fl.LogRecordBody }, matcher)
}

func HaveObservedTimestamp(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogOTel) string { return fl.ObservedTimestamp }, matcher)
}

func HaveOTelTimestamp(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogOTel) string { return fl.Timestamp }, matcher)
}

func HaveTraceId(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogOTel) string { return fl.TraceId }, matcher)
}

func HaveSpanId(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogOTel) string { return fl.SpanId }, matcher)
}

func HaveTraceFlags(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogOTel) uint32 { return fl.TraceFlags }, matcher)
}

func HaveSeverityText(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogOTel) string { return fl.SeverityText }, matcher)
}

func HaveSeverityNumber(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogOTel) int { return fl.SeverityNumber }, matcher)
}
