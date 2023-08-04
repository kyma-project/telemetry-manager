package matchers

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"go.opentelemetry.io/collector/pdata/plog"
)

const (
	tagNamespace = "namespace_name"
	tagPod       = "pod_name"
	tagContainer = "container_name"
)

type ResourceTags struct {
	Namespace string
	Pod       string
	Container string
}

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

type LogFilter func(lr *plog.LogRecord) bool

func WithNamespace(expectedNamespace string) LogFilter {
	return func(lr *plog.LogRecord) bool {
		kubernetesAttrs, hasKubernetesAttrs := lr.Attributes().AsRaw()["kubernetes"].(map[string]any)
		if !hasKubernetesAttrs {
			return false
		}
		namespace, hasNamespace := kubernetesAttrs[tagNamespace]
		if !hasNamespace {
			return false
		}
		namespaceStr, isStr := namespace.(string)
		if !isStr {
			return false
		}
		return strings.HasPrefix(namespaceStr, expectedNamespace)
	}
}

func WithPod(expectedPod string) LogFilter {
	return func(lr *plog.LogRecord) bool {
		kubernetesAttrs, hasKubernetesAttrs := lr.Attributes().AsRaw()["kubernetes"].(map[string]any)
		if !hasKubernetesAttrs {
			return false
		}
		pod, hasPod := kubernetesAttrs[tagPod]
		if !hasPod {
			return false
		}
		podStr, isStr := pod.(string)
		if !isStr {
			return false
		}
		return strings.HasPrefix(podStr, expectedPod)
	}
}

func WithContainer(expectedContainer string) LogFilter {
	return func(lr *plog.LogRecord) bool {
		kubernetesAttrs, hasKubernetesAttrs := lr.Attributes().AsRaw()["kubernetes"].(map[string]any)
		if !hasKubernetesAttrs {
			return false
		}
		container, hasContainer := kubernetesAttrs[tagContainer]
		if !hasContainer {
			return false
		}
		containerStr, isStr := container.(string)
		if !isStr {
			return false
		}
		return strings.HasPrefix(containerStr, expectedContainer)
	}
}

func WithAttributeKeyValue(expectedKey, expectedValue string) LogFilter {
	return func(lr *plog.LogRecord) bool {
		attr, hasAttr := lr.Attributes().AsRaw()[expectedKey].(string)
		if !hasAttr {
			return false
		}

		return attr == expectedValue
	}
}

func WithKubernetesLabels() LogFilter {
	return func(lr *plog.LogRecord) bool {
		kubernetesAttrs, hasKubernetesAttrs := lr.Attributes().AsRaw()["kubernetes"].(map[string]any)
		if !hasKubernetesAttrs {
			return false
		}

		_, hasLabels := kubernetesAttrs["labels"]
		return hasLabels
	}
}

func WithKubernetesAnnotations() LogFilter {
	return func(lr *plog.LogRecord) bool {
		kubernetesAttrs, hasKubernetesAttrs := lr.Attributes().AsRaw()["kubernetes"].(map[string]any)
		if !hasKubernetesAttrs {
			return false
		}

		_, hasLabels := kubernetesAttrs["annotations"]
		return hasLabels
	}
}

// ContainLogs succeeds if the filexporter output file contains any logs with the Kubernetes attributes passed into the matcher.
func ContainLogs(filters ...LogFilter) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlLogs []byte) (bool, error) {
		lds, err := unmarshalLogs(jsonlLogs)
		if err != nil {
			return false, fmt.Errorf("ContainLogs requires a valid OTLP JSON document: %v", err)
		}

		logRecords := getAllLogRecords(lds)
		for _, lr := range logRecords {
			if len(filters) == 0 {
				return true, nil
			}

			for _, filter := range filters {
				if filter(&lr) {
					return true, nil
				}
			}
		}
		return false, nil
	}, gomega.BeTrue())
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
	scanner := bufio.NewScanner(bytes.NewReader(buffer))
	// default buffer size causing 'token too long' error, buffer size configured for current test scenarios
	scannerBuffer := make([]byte, 0, 64*1024)
	scanner.Buffer(scannerBuffer, 1024*1024)

	for scanner.Scan() {
		td, err := logsUnmarshaler.UnmarshalLogs(scanner.Bytes())
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshall logs: %v", err)
		}

		lds = append(lds, td)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read logs: %v", err)
	}

	return lds, nil
}
