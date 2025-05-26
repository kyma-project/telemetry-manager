package fluentbit

import (
	"fmt"
	"time"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

const iso8601 = "2006-01-02T15:04:05.999Z"

func HaveFlatLogs(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlPLogs []byte) ([]FlatLog, error) {
		lds, err := unmarshalPLogs(jsonlPLogs)
		if err != nil {
			return nil, fmt.Errorf("HaveFlat requires a valid OTLP JSON document: %w", err)
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
