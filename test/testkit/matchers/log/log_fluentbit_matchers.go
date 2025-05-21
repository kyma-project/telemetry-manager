package log

import (
	"fmt"
	"time"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

const iso8601 = "2006-01-02T15:04:05.999Z"

func HaveFlatFluentBitLogs(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonLogs []byte) ([]FlatLogFluentBit, error) {
		lds, err := unmarshalFluentBitLogs(jsonLogs)
		if err != nil {
			return nil, fmt.Errorf("HaveFlatFluentBitLogs requires a valid OTLP JSON document: %w", err)
		}

		fl := flattenAllFluentBitLogs(lds)

		return fl, nil
	}, matcher)
}

func HaveContainerName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogFluentBit) string {
		return fl.KubernetesAttributes["container_name"]
	}, matcher)
}

func HaveNamespace(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogFluentBit) string {
		return fl.KubernetesAttributes["namespace_name"]
	}, matcher)
}

func HavePodName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogFluentBit) string {
		return fl.KubernetesAttributes["pod_name"]
	}, matcher)
}

func HaveLogRecordAttributes(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogFluentBit) map[string]string {
		return fl.LogRecordAttributes
	}, matcher)
}

func HaveTimestamp(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogFluentBit) time.Time {
		ts := fl.LogRecordAttributes["timestamp"]
		timestamp, err := time.Parse(time.RFC3339, ts)

		if err != nil {
			panic(err)
		}

		return timestamp
	}, matcher)
}

func HaveLevel(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogFluentBit) string {
		return fl.LogRecordAttributes["level"]
	}, matcher)
}

func HaveKubernetesAnnotations(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogFluentBit) map[string]any {
		return fl.KubernetesAnnotationAttributes
	}, matcher)
}

func HaveKubernetesLabels(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogFluentBit) map[string]any {
		return fl.KubernetesLabelAttributes
	}, matcher)
}

func HaveLogBody(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogFluentBit) string { return fl.LogRecordBody }, matcher)
}

func HaveDateISO8601Format(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogFluentBit) bool {
		date := fl.LogRecordAttributes["date"]
		_, err := time.Parse(iso8601, date)

		return err == nil
	}, matcher)
}
