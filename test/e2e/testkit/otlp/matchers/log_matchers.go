package matchers

import (
	"bufio"
	"bytes"
	"fmt"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"go.opentelemetry.io/collector/pdata/plog"
)

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
