package log

import (
	"fmt"
	"time"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

func HaveFlatHTTPLogs(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonLogs []byte) ([]FlatLogHTTP, error) {
		lds, err := unmarshalHTTPLogs(jsonLogs)
		if err != nil {
			return nil, fmt.Errorf("HaveFlatHTTPLogs requires a valid OTLP JSON document: %w", err)
		}

		fl := flattenAllHTTPLogs(lds)

		return fl, nil
	}, matcher)
}

func HaveContainerName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogHTTP) string {
		return fl.KubernetesAttributes["container_name"]
	}, matcher)
}

func HaveNamespace(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogHTTP) string {
		return fl.KubernetesAttributes["namespace_name"]
	}, matcher)
}

func HavePodName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogHTTP) string {
		return fl.KubernetesAttributes["pod_name"]
	}, matcher)
}

func HaveLogRecordAttributes(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogHTTP) map[string]string {
		return fl.LogRecordAttributes
	}, matcher)
}

func HaveTimestamp(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogHTTP) time.Time {
		ts := fl.LogRecordAttributes["timestamp"]
		timestamp, err := time.Parse(time.RFC3339, ts)

		if err != nil {
			panic(err)
		}

		return timestamp
	}, matcher)
}

func HaveLevel(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogHTTP) string {
		return fl.LogRecordAttributes["level"]
	}, matcher)
}

func HaveKubernetesAnnotations(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogHTTP) map[string]any {
		return fl.KubernetesAnnotationAttributes
	}, matcher)
}

func HaveKubernetesLabels(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogHTTP) map[string]any {
		return fl.KubernetesLabelAttributes
	}, matcher)
}

func HaveLogBody(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLogHTTP) string {
		return fl.LogRecordBody
	}, matcher)
}
