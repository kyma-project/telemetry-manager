package log

import (
	"fmt"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func WithLds(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlLogs []byte) ([]plog.Logs, error) {
		lds, err := unmarshalLogs(jsonlLogs)
		if err != nil {
			return nil, fmt.Errorf("WithLds requires a valid OTLP JSON document: %v", err)
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

func WithContainerName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(lr plog.LogRecord) string {
		const kubernetesAttrKey = "kubernetes"
		kubernetesAttrs, hasKubernetesAttrs := lr.Attributes().Get(kubernetesAttrKey)
		if !hasKubernetesAttrs || kubernetesAttrs.Type() != pcommon.ValueTypeMap {
			return ""
		}

		const containerNameAttrKey = "container_name"
		containerName, hasContainerName := kubernetesAttrs.Map().Get(containerNameAttrKey)
		if !hasContainerName || containerName.Type() != pcommon.ValueTypeStr {
			return ""
		}

		return containerName.Str()
	}, matcher)
}

func WithPodName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(lr plog.LogRecord) string {
		const kubernetesAttrKey = "kubernetes"
		kubernetesAttrs, hasKubernetesAttrs := lr.Attributes().Get(kubernetesAttrKey)
		if !hasKubernetesAttrs || kubernetesAttrs.Type() != pcommon.ValueTypeMap {
			return ""
		}

		const podNameAttrKey = "pod_name"
		podName, hasPodName := kubernetesAttrs.Map().Get(podNameAttrKey)
		if !hasPodName || podName.Type() != pcommon.ValueTypeStr {
			return ""
		}

		return podName.Str()
	}, matcher)
}

func WithKubernetesAnnotations(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(lr plog.LogRecord) map[string]any {
		const kubernetesAttrKey = "kubernetes"
		kubernetesAttrs, hasKubernetesAttrs := lr.Attributes().Get(kubernetesAttrKey)
		if !hasKubernetesAttrs || kubernetesAttrs.Type() != pcommon.ValueTypeMap {
			return nil
		}
		annotationAttrs, hasAnnotations := kubernetesAttrs.Map().Get("annotations")
		if !hasAnnotations || kubernetesAttrs.Type() != pcommon.ValueTypeMap {
			return nil
		}
		return annotationAttrs.Map().AsRaw()
	}, matcher)
}

func WithKubernetesLabels(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(lr plog.LogRecord) map[string]any {
		const kubernetesAttrKey = "kubernetes"
		kubernetesAttrs, hasKubernetesAttrs := lr.Attributes().Get(kubernetesAttrKey)
		if !hasKubernetesAttrs || kubernetesAttrs.Type() != pcommon.ValueTypeMap {
			return nil
		}
		annotationAttrs, hasAnnotations := kubernetesAttrs.Map().Get("labels")
		if !hasAnnotations || kubernetesAttrs.Type() != pcommon.ValueTypeMap {
			return nil
		}
		return annotationAttrs.Map().AsRaw()
	}, matcher)
}
