package log

import (
	"fmt"
	"time"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

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

func WithLds(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlLogs []byte) ([]plog.Logs, error) {
		if jsonlLogs == nil {
			return nil, nil
		}

		lds, err := unmarshalLogs(jsonlLogs)
		if err != nil {
			return nil, fmt.Errorf("WithLds requires a valid OTLP JSON document: %w", err)
		}

		return lds, nil
	}, matcher)
}

// ContainLd is an alias for WithLds(gomega.ContainElement()).
func ContainLd(matcher types.GomegaMatcher) types.GomegaMatcher {
	return WithLds(gomega.ContainElement(matcher))
}

// ConsistOfLds is an alias for WithLds(gomega.ConsistOf()).
func ConsistOfLds(matcher types.GomegaMatcher) types.GomegaMatcher {
	return WithLds(gomega.ConsistOf(matcher))
}

func WithLogRecords(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(ld plog.Logs) ([]plog.LogRecord, error) {
		return getLogRecords(ld), nil
	}, matcher)
}

// ContainLogRecord is an alias for WithLogRecords(gomega.ContainElement()).
func ContainLogRecord(matcher types.GomegaMatcher) types.GomegaMatcher {
	return WithLogRecords(gomega.ContainElement(matcher))
}

func HaveContainerName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string {
		return fl.ContainerName
	}, matcher)
}

func WithContainerName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(lr plog.LogRecord) string {
		kubernetesAttrs := getKubernetesAttributes(lr)
		containerName, hasContainerName := kubernetesAttrs.Get("container_name")
		if !hasContainerName || containerName.Type() != pcommon.ValueTypeStr {
			return ""
		}

		return containerName.Str()
	}, matcher)
}

func HaveNamespace(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string {
		return fl.NamespaceName
	}, matcher)
}

func HaveLogRecordAttributes(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) map[string]string {
		return fl.LogRecordAttributes
	}, matcher)

}

func HaveTimestamp(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) time.Time {
		return fl.Timestamp
	}, matcher)
}

func HavePodName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string {
		return fl.PodName
	}, matcher)
}

func WithPodName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(lr plog.LogRecord) string {
		kubernetesAttrs := getKubernetesAttributes(lr)
		podName, hasPodName := kubernetesAttrs.Get("pod_name")
		if !hasPodName || podName.Type() != pcommon.ValueTypeStr {
			return ""
		}

		return podName.Str()
	}, matcher)
}

func HaveLevel(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string {
		return fl.Level
	}, matcher)
}

func WithLevel(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(lr plog.LogRecord) string {
		const levelAttrKey = "level"
		levelAttr, hasLevelAttr := lr.Attributes().Get(levelAttrKey)
		if !hasLevelAttr || levelAttr.Type() != pcommon.ValueTypeStr {
			return ""
		}

		return levelAttr.Str()
	}, matcher)
}

func WithTimestamp(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(lr plog.LogRecord) time.Time {
		const timestampAttrKey = "timestamp"
		timestampAttr, hasTimestampAttr := lr.Attributes().Get(timestampAttrKey)
		if !hasTimestampAttr || timestampAttr.Type() != pcommon.ValueTypeStr {
			return time.Time{}
		}

		timestamp, err := time.Parse(time.RFC3339, timestampAttr.Str())
		if err != nil {
			return time.Time{}
		}

		return timestamp
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

func WithKubernetesAnnotations(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(lr plog.LogRecord) map[string]any {
		kubernetesAttrs := getKubernetesAttributes(lr)
		annotationAttrs, hasAnnotations := kubernetesAttrs.Get("annotations")
		if !hasAnnotations || annotationAttrs.Type() != pcommon.ValueTypeMap {
			return nil
		}
		return annotationAttrs.Map().AsRaw()
	}, matcher)
}

func WithKubernetesLabels(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(lr plog.LogRecord) map[string]any {
		kubernetesAttrs := getKubernetesAttributes(lr)
		labelAttrs, hasLabels := kubernetesAttrs.Get("labels")
		if !hasLabels || labelAttrs.Type() != pcommon.ValueTypeMap {
			return nil
		}
		return labelAttrs.Map().AsRaw()
	}, matcher)
}

func WithLogRecordAttrs(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(lr plog.LogRecord) map[string]any {
		return lr.Attributes().AsRaw()
	}, matcher)
}

func HaveLogBody(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fl FlatLog) string {
		return fl.LogRecordBody
	}, matcher)
}

func WithLogBody(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(lr plog.LogRecord) string {
		return lr.Body().AsString()
	}, matcher)
}
