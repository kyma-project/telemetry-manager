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

func ContainLogs() types.GomegaMatcher {
	return gomega.WithTransform(func(actual interface{}) (int, error) {
		if actual == nil {
			return 0, nil
		}

		actualBytes, ok := actual.([]byte)
		if !ok {
			return 0, fmt.Errorf("ContainLogs requires a []byte, but got %T", actual)
		}

		actualLogs, err := unmarshalOTLPJSONLogs(actualBytes)
		if err != nil {
			return 0, fmt.Errorf("ContainLogs requires a valid OTLP JSON document: %v", err)
		}

		actualLogRecords := getAllLogRecords(actualLogs)
		return len(actualLogRecords), nil
	}, gomega.BeNumerically(">", 0))
}

func ConsistOfNumberOfLogs(count int) types.GomegaMatcher {
	return gomega.WithTransform(func(actual interface{}) (int, error) {
		if actual == nil {
			return 0, nil
		}

		actualBytes, ok := actual.([]byte)
		if !ok {
			return 0, fmt.Errorf("ConsistOfNumberOfLogs requires a []byte, but got %T", actual)
		}

		actualLogs, err := unmarshalOTLPJSONLogs(actualBytes)
		if err != nil {
			return 0, fmt.Errorf("ConsistOfNumberOfLogs requires a valid OTLP JSON document: %v", err)
		}

		actualLogRecords := getAllLogRecords(actualLogs)

		return len(actualLogRecords), nil
	}, gomega.Equal(count))
}

func ContainsLogsWith(namespace, pod, container string) types.GomegaMatcher {
	return gomega.WithTransform(func(actual interface{}) (bool, error) {
		filter := ResourceTags{
			Namespace: namespace,
			Pod:       pod,
			Container: container,
		}

		actualBytes, ok := actual.([]byte)
		if !ok {
			return false, fmt.Errorf("ContainsLogsWith requires a []byte, but got %T", actual)
		}

		actualLogs, err := unmarshalOTLPJSONLogs(actualBytes)
		if err != nil {
			return false, fmt.Errorf("ContainsLogsWith requires a valid OTLP JSON document: %v", err)
		}

		actualLogRecords := getAllLogRecords(actualLogs)

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
