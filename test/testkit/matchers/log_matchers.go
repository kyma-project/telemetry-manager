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

type LogFilter func(lr *plog.LogRecord) (bool, error)

func WithNamespace(expectedNamespace string) LogFilter {
	return func(lr *plog.LogRecord) (bool, error) {
		kubernetesAttrs, hasKubernetesAttrs := lr.Attributes().AsRaw()["kubernetes"].(map[string]any)
		if !hasKubernetesAttrs {
			return false, nil
		}
		namespace, hasNamespace := kubernetesAttrs[tagNamespace]
		if !hasNamespace {
			return false, nil
		}
		namespaceStr, isStr := namespace.(string)
		if !isStr {
			return false, nil
		}
		return strings.HasPrefix(namespaceStr, expectedNamespace), nil
	}
}

func WithPod(expectedPod string) LogFilter {
	return func(lr *plog.LogRecord) (bool, error) {
		kubernetesAttrs, hasKubernetesAttrs := lr.Attributes().AsRaw()["kubernetes"].(map[string]any)
		if !hasKubernetesAttrs {
			return false, nil
		}
		pod, hasPod := kubernetesAttrs[tagPod]
		if !hasPod {
			return false, nil
		}
		podStr, isStr := pod.(string)
		if !isStr {
			return false, nil
		}
		return strings.HasPrefix(podStr, expectedPod), nil
	}
}

func WithContainer(expectedContainer string) LogFilter {
	return func(lr *plog.LogRecord) (bool, error) {
		kubernetesAttrs, hasKubernetesAttrs := lr.Attributes().AsRaw()["kubernetes"].(map[string]any)
		if !hasKubernetesAttrs {
			return false, nil
		}
		container, hasContainer := kubernetesAttrs[tagNamespace]
		if !hasContainer {
			return false, nil
		}
		containerStr, isStr := container.(string)
		if !isStr {
			return false, nil
		}
		return strings.HasPrefix(containerStr, expectedContainer), nil
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
				match, filterErr := filter(&lr)
				if filterErr != nil {
					return false, filterErr
				}
				if match {
					return true, nil
				}
			}
		}
		return false, nil
	}, gomega.BeTrue())
}

// ContainLogsWithAttribute succeeds if the filexporter output file contains any logs with the string attribute passed into the matcher.
func ContainLogsWithAttribute(key, value string) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlLogs []byte) (bool, error) {
		logs, err := unmarshalLogs(jsonlLogs)
		if err != nil {
			return false, fmt.Errorf("ContainLogsWithAttribute requires a valid OTLP JSON document: %v", err)
		}

		logRecords := getAllLogRecords(logs)

		for _, lr := range logRecords {
			attribute, ok := lr.Attributes().AsRaw()[key].(string)
			if !ok {
				continue
			}

			if attribute == value {
				return true, nil
			}
		}
		return false, nil
	}, gomega.BeTrue())
}

// ContainLogsWithKubernetesLabels succeeds if the filexporter output file contains any logs with Kubernetes labels.
func ContainLogsWithKubernetesLabels() types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlLogs []byte) (bool, error) {
		logs, err := unmarshalLogs(jsonlLogs)
		if err != nil {
			return false, fmt.Errorf("ContainLogsWithKubernetesLabels requires a valid OTLP JSON document: %v", err)
		}

		logRecords := getAllLogRecords(logs)
		if len(logRecords) == 0 {
			return false, nil
		}

		for _, lr := range logRecords {
			k8sAttributes, hasKubernetes := lr.Attributes().AsRaw()["kubernetes"].(map[string]any)
			if !hasKubernetes {
				continue
			}

			_, hasLabels := k8sAttributes["labels"]
			if hasLabels {
				return true, nil
			}
		}
		return false, nil
	}, gomega.BeTrue())
}

// ContainLogsWithKubernetesAnnotations succeeds if the filexporter output file contains any logs with Kubernetes annotations.
func ContainLogsWithKubernetesAnnotations() types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlLogs []byte) (bool, error) {
		logs, err := unmarshalLogs(jsonlLogs)
		if err != nil {
			return false, fmt.Errorf("ContainLogs requires a valid OTLP JSON document: %v", err)
		}

		logRecords := getAllLogRecords(logs)
		if len(logRecords) == 0 {
			return false, nil
		}

		for _, lr := range logRecords {
			k8sAttributes, hasKubernetes := lr.Attributes().AsRaw()["kubernetes"].(map[string]any)
			if !hasKubernetes {
				continue
			}

			_, hasAnnotations := k8sAttributes["annotations"]
			if hasAnnotations {
				return true, nil
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
