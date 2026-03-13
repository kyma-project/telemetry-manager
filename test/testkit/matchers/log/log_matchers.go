package log

import (
	"fmt"

	"github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
)

func HaveFlatLogs(matcher gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return gomega.WithTransform(func(jsonLogs []byte) ([]FlatLog, error) {
		tds, err := unmarshalLogs(jsonLogs)
		if err != nil {
			return nil, fmt.Errorf("HaveFlatLogs requires a valid OTLP JSON document: %w", err)
		}

		ft := flattenAllLogs(tds)

		return ft, nil
	}, matcher)
}

// HaveResourceAttributes extracts resource attributes from FlatLog and applies the matcher to them.
func HaveResourceAttributes(matcher gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) map[string]string {
		return fl.ResourceAttributes
	}, matcher)
}

// HaveScopeName extracts scope name from FlatLog and applies the matcher to it.
func HaveScopeName(matcher gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string {
		return fl.ScopeName
	}, matcher)
}

// HaveScopeVersion extracts scope version from FlatLog and applies the matcher to it.
func HaveScopeVersion(matcher gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string {
		return fl.ScopeVersion
	}, matcher)
}

// HaveAttributes extracts resource attributes from FlatLog and applies the matcher to them.
func HaveAttributes(matcher gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) map[string]string { return fl.Attributes }, matcher)
}

func HaveLogBody(matcher gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string { return fl.LogRecordBody }, matcher)
}

func HaveObservedTimestamp(matcher gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string { return fl.ObservedTimestamp }, matcher)
}

func HaveTimestamp(matcher gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string { return fl.Timestamp }, matcher)
}

func HaveTraceID(matcher gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string { return fl.TraceId }, matcher)
}

func HaveSpanID(matcher gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string { return fl.SpanId }, matcher)
}

func HaveTraceFlags(matcher gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) uint32 { return fl.TraceFlags }, matcher)
}

func HaveSeverityText(matcher gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string { return fl.SeverityText }, matcher)
}

func HaveSeverityNumber(matcher gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) int { return fl.SeverityNumber }, matcher)
}
