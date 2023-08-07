package matchers

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"io"
)

// ConsistOfNumberOfLogs succeeds if the filexporter output file has the expected number of logs.
func ConsistOfNumberOfLogs(count int) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlLogs []byte) (int, error) {
		lds, err := unmarshalLogs(jsonlLogs)
		if err != nil {
			return 0, fmt.Errorf("ConsistOfNumberOfLogs requires a valid OTLP JSON document: %v", err)
		}

		logRecords := getAllLogRecords(lds)

		return len(logRecords), nil
	}, gomega.Equal(count))
}

type LogFilter func(lr plog.LogRecord) bool

// ContainLogs succeeds if the filexporter output file contains any logs that matches the log filter passed into the matcher.
func ContainLogs(f LogFilter) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlLogs []byte) (bool, error) {
		lds, err := unmarshalLogs(jsonlLogs)
		if err != nil {
			return false, fmt.Errorf("ContainLogs requires a valid OTLP JSON document: %v", err)
		}

		logRecords := getAllLogRecords(lds)
		for _, lr := range logRecords {
			if f(lr) {
				return true, nil
			}
		}
		return false, nil
	}, gomega.BeTrue())
}

func Any() LogFilter {
	return func(plog.LogRecord) bool {
		return true
	}
}

func WithNamespace(expectedNamespace string) LogFilter {
	return func(lr plog.LogRecord) bool {
		const namespaceAttrKey = "namespace_name"
		namespace, exists := getKubernetesAttributeValue(lr, namespaceAttrKey)
		if !exists {
			return false
		}
		return strings.HasPrefix(namespace, expectedNamespace)
	}
}

func WithPod(expectedPod string) LogFilter {
	return func(lr plog.LogRecord) bool {
		const podAttrKey = "pod_name"
		pod, exists := getKubernetesAttributeValue(lr, podAttrKey)
		if !exists {
			return false
		}
		return strings.HasPrefix(pod, expectedPod)
	}
}

func WithContainer(expectedContainer string) LogFilter {
	return func(lr plog.LogRecord) bool {
		const containerAttrKey = "container_name"
		container, exists := getKubernetesAttributeValue(lr, containerAttrKey)
		if !exists {
			return false
		}
		return strings.HasPrefix(container, expectedContainer)
	}
}

func WithAttributeKeyValue(expectedKey, expectedValue string) LogFilter {
	return func(lr plog.LogRecord) bool {
		attr, hasAttr := lr.Attributes().Get(expectedKey)
		if !hasAttr || attr.Type() != pcommon.ValueTypeStr {
			return false
		}

		return attr.Str() == expectedValue
	}
}

func WithAttributeKeys(expectedKeys ...string) LogFilter {
	return func(lr plog.LogRecord) bool {
		for _, k := range expectedKeys {
			_, hasAttr := lr.Attributes().Get(k)
			if !hasAttr {
				return false
			}
		}
		return true
	}
}

func WithKubernetesLabels() LogFilter {
	return func(lr plog.LogRecord) bool {
		kubernetesAttrs, hasKubernetesAttrs := getKubernetesAttributes(lr)
		if !hasKubernetesAttrs {
			return false
		}

		_, hasLabels := kubernetesAttrs.Get("labels")
		return hasLabels
	}
}

func WithKubernetesAnnotations() LogFilter {
	return func(lr plog.LogRecord) bool {
		kubernetesAttrs, hasKubernetesAttrs := getKubernetesAttributes(lr)
		if !hasKubernetesAttrs {
			return false
		}

		_, hasAnnotations := kubernetesAttrs.Get("annotations")
		return hasAnnotations
	}
}

func getKubernetesAttributeValue(lr plog.LogRecord, attrKey string) (string, bool) {
	kubernetesAttrs, hasKubernetesAttrs := getKubernetesAttributes(lr)
	if !hasKubernetesAttrs {
		return "", false
	}

	attrValue, hasAttr := kubernetesAttrs.Get(attrKey)
	if !hasAttr || attrValue.Type() != pcommon.ValueTypeStr {
		return "", false
	}

	return attrValue.Str(), true
}

func getKubernetesAttributes(lr plog.LogRecord) (pcommon.Map, bool) {
	const kubernetesAttrKey = "kubernetes"
	kubernetesAttrs, hasKubernetesAttrs := lr.Attributes().Get(kubernetesAttrKey)
	if !hasKubernetesAttrs || kubernetesAttrs.Type() != pcommon.ValueTypeMap {
		return pcommon.NewMap(), false
	}
	return kubernetesAttrs.Map(), true
}

func getAllLogRecords(lds []plog.Logs) []plog.LogRecord {
	var logRecords []plog.LogRecord

	for _, lr := range lds {
		for i := 0; i < lr.ResourceLogs().Len(); i++ {
			resourceLogs := lr.ResourceLogs().At(i)
			for j := 0; j < resourceLogs.ScopeLogs().Len(); j++ {
				scopeLogs := resourceLogs.ScopeLogs().At(j)
				for k := 0; k < scopeLogs.LogRecords().Len(); k++ {
					logRecords = append(logRecords, scopeLogs.LogRecords().At(k))
				}
			}
		}
	}

	return logRecords
}

func unmarshalLogs(buffer []byte) ([]plog.Logs, error) {
	var lds []plog.Logs

	var logsUnmarshaler plog.JSONUnmarshaler

	// User bufio.Reader instead of bufio.Scanner to handle very long lines gracefully
	reader := bufio.NewReader(bytes.NewReader(buffer))
	for {
		line, readerErr := reader.ReadBytes('\n')
		if readerErr != nil && readerErr != io.EOF {
			return nil, fmt.Errorf("failed to read line: %v", readerErr)
		}

		if len(line) > 0 {
			ld, err := logsUnmarshaler.UnmarshalLogs(line)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal logs: %v", readerErr)
			}
			lds = append(lds, ld)
		}

		if readerErr == io.EOF {
			break
		}
	}

	return lds, nil
}
