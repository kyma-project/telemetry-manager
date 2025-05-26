package fluentbit

import (
	"fmt"
	"time"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

const iso8601 = "2006-01-02T15:04:05.999Z"

// HaveFlatLogs unmarshals OTLP logs from a JSON file, converts them to FlatLogs and applies the matcher to them.
// Even though FluentBit does not use OTLP, the logs are still in OTLP format, so we can use the e2e test setup as for other telemetry types.
func HaveFlatLogs(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonLogs []byte) ([]FlatLog, error) {
		lds, err := unmarshalLogs(jsonLogs)
		if err != nil {
			return nil, fmt.Errorf("HaveFlatLogs requires a valid OTLP JSON document: %w", err)
		}

		fl := flattenAllLogs(lds)

		return fl, nil
	}, matcher)
}

func HaveContainerName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string {
		return fl.KubernetesAttributes["container_name"]
	}, matcher)
}

func HaveNamespace(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string {
		return fl.KubernetesAttributes["namespace_name"]
	}, matcher)
}

func HavePodName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string {
		return fl.KubernetesAttributes["pod_name"]
	}, matcher)
}

func HaveKubernetesAttributes(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) map[string]string {
		return fl.KubernetesAttributes
	}, matcher)
}

func HaveAttributes(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) map[string]string {
		return fl.Attributes
	}, matcher)
}

func HaveTimestamp(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) time.Time {
		ts := fl.Attributes["timestamp"]
		timestamp, err := time.Parse(time.RFC3339, ts)

		if err != nil {
			panic(err)
		}

		return timestamp
	}, matcher)
}

func HaveLevel(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string {
		return fl.Attributes["level"]
	}, matcher)
}

func HaveKubernetesAnnotations(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) map[string]any {
		return fl.KubernetesAnnotationAttributes
	}, matcher)
}

func HaveKubernetesLabels(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) map[string]any {
		return fl.KubernetesLabelAttributes
	}, matcher)
}

func HaveLogBody(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string { return fl.LogBody }, matcher)
}

func HaveDateISO8601Format(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) bool {
		date := fl.Attributes["date"]
		_, err := time.Parse(iso8601, date)

		return err == nil
	}, matcher)
}
