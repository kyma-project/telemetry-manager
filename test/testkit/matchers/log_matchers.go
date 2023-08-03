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

// ContainLogs succeeds if the filexporter output file has any logs.
func ContainLogs() types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlLogs []byte) (int, error) {
		lds, err := unmarshalLogs(jsonlLogs)
		if err != nil {
			return 0, fmt.Errorf("ContainLogs requires a valid OTLP JSON document: %v", err)
		}

		logRecords := getAllLogRecords(lds)

		return len(logRecords), nil
	}, gomega.BeNumerically(">", 0))
}

// ContainLogsWithKubernetesAttributes succeeds if the filexporter output file contains any logs with the Kubernetes attributes passed into the matcher.
func ContainLogsWithKubernetesAttributes(namespace, pod, container string) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlLogs []byte) (bool, error) {
		lds, err := unmarshalLogs(jsonlLogs)
		if err != nil {
			return false, fmt.Errorf("ContainLogsWithKubernetesAttributes requires a valid OTLP JSON document: %v", err)
		}

		logRecords := getAllLogRecords(lds)

		filter := ResourceTags{
			Namespace: namespace,
			Pod:       pod,
			Container: container,
		}
		for _, lr := range logRecords {
			attributes, hasKubernetes := lr.Attributes().AsRaw()["kubernetes"].(map[string]any)
			if !hasKubernetes {
				continue
			}
			tags, err := extractTags(attributes)
			if err != nil {
				return false, fmt.Errorf("LogRecord has invalid or malformed attributes: %v", err)
			}

			if matchPrefixes(tags, filter) {
				return true, nil
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
			return false, fmt.Errorf("ContainLogsWithKubernetesAttributes requires a valid OTLP JSON document: %v", err)
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

func matchPrefixes(logRecordTags, filter ResourceTags) bool {
	if filter.Namespace != "" && !strings.HasPrefix(logRecordTags.Namespace, filter.Namespace) {
		return false
	}

	if filter.Pod != "" && !strings.HasPrefix(logRecordTags.Pod, filter.Pod) {
		return false
	}

	if filter.Container != "" && !strings.HasPrefix(logRecordTags.Container, filter.Container) {
		return false
	}

	return true
}

func extractTags(attrs map[string]any) (tags ResourceTags, err error) {
	for k, v := range attrs {
		if k != tagNamespace && k != tagPod && k != tagContainer {
			continue
		}

		tagValue, ok := v.(string)
		if !ok {
			return tags, fmt.Errorf("an attribute %s is malformed", k)
		}

		switch k {
		case tagNamespace:
			tags.Namespace = tagValue
		case tagPod:
			tags.Pod = tagValue
		case tagContainer:
			tags.Container = tagValue
		}
	}

	return tags, nil
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
