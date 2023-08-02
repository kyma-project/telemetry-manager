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
	return gomega.WithTransform(func(fileBytes []byte) (int, error) {
		actualLds, err := unmarshalOTLPJSONLogs(fileBytes)
		if err != nil {
			return 0, fmt.Errorf("ConsistOfNumberOfLogs requires a valid OTLP JSON document: %v", err)
		}

		actualLogRecords := getAllLogRecords(actualLds)

		return len(actualLogRecords), nil
	}, gomega.Equal(count))
}

// ContainLogs succeeds if the filexporter output file has any logs.
func ContainLogs() types.GomegaMatcher {
	return gomega.WithTransform(func(fileBytes []byte) (int, error) {
		actualLds, err := unmarshalOTLPJSONLogs(fileBytes)
		if err != nil {
			return 0, fmt.Errorf("ContainLogs requires a valid OTLP JSON document: %v", err)
		}

		actualLogRecords := getAllLogRecords(actualLds)

		return len(actualLogRecords), nil
	}, gomega.BeNumerically(">", 0))
}

// ContainLogsWithKubernetesAttributes succeeds if the filexporter output file contains any logs with the Kubernetes attributes passed into the matcher.
func ContainLogsWithKubernetesAttributes(namespace, pod, container string) types.GomegaMatcher {
	return gomega.WithTransform(func(fileBytes []byte) (bool, error) {
		actualLds, err := unmarshalOTLPJSONLogs(fileBytes)
		if err != nil {
			return false, fmt.Errorf("ContainLogsWithKubernetesAttributes requires a valid OTLP JSON document: %v", err)
		}

		actualLogRecords := getAllLogRecords(actualLds)

		filter := ResourceTags{
			Namespace: namespace,
			Pod:       pod,
			Container: container,
		}
		for _, lr := range actualLogRecords {
			attributes, ok := lr.Attributes().AsRaw()["kubernetes"].(map[string]any)
			if !ok {
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

// ContainsLogsWithAttribute succeeds if the filexporter output file contains any logs with the string attribute passed into the matcher.
func ContainsLogsWithAttribute(key, value string) types.GomegaMatcher {
	return gomega.WithTransform(func(fileBytes []byte) (bool, error) {
		actualLogs, err := unmarshalOTLPJSONLogs(fileBytes)
		if err != nil {
			return false, fmt.Errorf("ContainsLogsWithAttribute requires a valid OTLP JSON document: %v", err)
		}

		actualLogRecords := getAllLogRecords(actualLogs)

		for _, lr := range actualLogRecords {
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

// ConsistOfLogsWithKubernetesLabels succeeds if the filexporter output file only consists of logs with Kubernetes annotations.
func ConsistOfLogsWithKubernetesLabels() types.GomegaMatcher {
	return gomega.WithTransform(func(fileBytes []byte) (bool, error) {
		actualLogs, err := unmarshalOTLPJSONLogs(fileBytes)
		if err != nil {
			return false, fmt.Errorf("ConsistOfLogsWithKubernetesLabels requires a valid OTLP JSON document: %v", err)
		}

		actualLogRecords := getAllLogRecords(actualLogs)
		if len(actualLogRecords) == 0 {
			return false, nil
		}

		for _, lr := range actualLogRecords {
			k8sAttributes, hasKubernetes := lr.Attributes().AsRaw()["kubernetes"].(map[string]any)
			if !hasKubernetes {
				return false, nil
			}

			_, hasLabels := k8sAttributes["labels"]
			if !hasLabels {
				return false, nil
			}
		}
		return true, nil
	}, gomega.BeTrue())
}

// ConsistOfLogsWithKubernetesAnnotations succeeds if the filexporter output file only consists of logs with Kubernetes annotations.
func ConsistOfLogsWithKubernetesAnnotations() types.GomegaMatcher {
	return gomega.WithTransform(func(fileBytes []byte) (bool, error) {
		actualLogs, err := unmarshalOTLPJSONLogs(fileBytes)
		if err != nil {
			return false, fmt.Errorf("ContainLogsWithKubernetesAttributes requires a valid OTLP JSON document: %v", err)
		}

		actualLogRecords := getAllLogRecords(actualLogs)
		if len(actualLogRecords) == 0 {
			return false, nil
		}

		for _, lr := range actualLogRecords {
			k8sAttributes, hasKubernetes := lr.Attributes().AsRaw()["kubernetes"].(map[string]any)
			if !hasKubernetes {
				return false, nil
			}

			_, hasAnnotations := k8sAttributes["annotations"]
			if !hasAnnotations {
				return false, nil
			}
		}
		return true, nil
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

func getAllLogRecords(logs []plog.Logs) []plog.LogRecord {
	var logRecords []plog.LogRecord

	for _, lr := range logs {
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

func unmarshalOTLPJSONLogs(buffer []byte) ([]plog.Logs, error) {
	var results []plog.Logs

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

		results = append(results, td)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read logs: %v", err)
	}

	return results, nil
}
