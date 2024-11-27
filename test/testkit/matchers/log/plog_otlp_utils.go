package log

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/kyma-project/telemetry-manager/test/testkit/matchers"
)

// FlatLogOtel holds all needed information about a Otel log record.
// Gomega doesn't handle deeply nested data structure very well and generates large, unreadable diffs when paired with the deeply nested structure of plogs.
//
// Introducing a go struct with a flat data structure by extracting necessary information from different levels of plogs makes accessing the information easier than using plog.
// Logs directly and improves the readability of the test output logs.
type FlatLogOtel struct {
	Name               string
	ResourceAttributes map[string]string
	LogRecordBody      string
}

func unmarshalOTLPLogs(jsonlMetrics []byte) ([]plog.Logs, error) {
	return matchers.UnmarshalSignals[plog.Logs](jsonlMetrics, func(buf []byte) (plog.Logs, error) {
		var unmarshaler plog.JSONUnmarshaler
		return unmarshaler.UnmarshalLogs(buf)
	})
}

// flattenAllOTLPLogs flattens an array of plog.Logs to a slice of FlatLogOTLP.
// It converts the deeply nested plog.Logs data structure to a flat struct, to make it more readable in the test output logs.
func flattenAllOTLPLogs(lds []plog.Logs) []FlatLogOtel {
	var flatLogs []FlatLogOtel

	for _, ld := range lds {
		flatLogs = append(flatLogs, flattenOTLPLogs(ld)...)
	}

	return flatLogs
}

// flattenOTLPLogs converts a single plog.Logs to a slice of FlatLogOTLP
// It takes relevant information from different levels of pdata and puts it into a FlatLogOTLP go struct.
func flattenOTLPLogs(ld plog.Logs) []FlatLogOtel {
	var flatLogs []FlatLogOtel

	for i := range ld.ResourceLogs().Len() {
		resourceLogs := ld.ResourceLogs().At(i)
		for j := range resourceLogs.ScopeLogs().Len() {
			scopeLogs := resourceLogs.ScopeLogs().At(j)
			for k := range scopeLogs.LogRecords().Len() {
				lr := scopeLogs.LogRecords().At(k)
				flatLogs = append(flatLogs, FlatLogOtel{
					ResourceAttributes: attributeToMapOTLP(resourceLogs.Resource().Attributes()),
					LogRecordBody:      lr.Body().AsString(),
				})
			}
		}
	}

	return flatLogs
}

// attributeToMap converts pdata.AttributeMap to a map using the string representation of the values.
func attributeToMapOTLP(attrs pcommon.Map) map[string]string {
	attrMap := make(map[string]string)

	attrs.Range(func(k string, v pcommon.Value) bool {
		attrMap[k] = v.AsString()
		return true
	})

	return attrMap
}
