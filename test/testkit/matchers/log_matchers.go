package matchers

import (
	"fmt"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

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

func unmarshalLogs(jsonlLogs []byte) ([]plog.Logs, error) {
	return UnmarshalSignals[plog.Logs](jsonlLogs, func(buf []byte) (plog.Logs, error) {
		var unmarshaler plog.JSONUnmarshaler
		return unmarshaler.UnmarshalLogs(buf)
	})
}
