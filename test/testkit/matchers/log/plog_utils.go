package log

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/kyma-project/telemetry-manager/test/testkit/matchers"
)

// FlatLog holds all needed information about an OTel log record.
// Gomega doesn't handle deeply nested data structure very well and generates large, unreadable diffs when paired with the deeply nested structure of plogs.
//
// Introducing a go struct with a flat data structure by extracting necessary information from different levels of plogs makes accessing the information easier than using plog.
// Logs directly and improves the readability of the test output logs.
type FlatLog struct {
	Name, ScopeName, ScopeVersion                                string
	ResourceAttributes, ScopeAttributes, Attributes              map[string]string
	LogRecordBody, ObservedTimestamp, Timestamp, TraceId, SpanId string
	SeverityText                                                 string
	SeverityNumber                                               int
	TraceFlags                                                   uint32
}

func unmarshalLogs(jsonLogs []byte) ([]plog.Logs, error) {
	return matchers.UnmarshalOTLPJSONData(jsonLogs, func(buf []byte) (plog.Logs, error) {
		var unmarshaler plog.JSONUnmarshaler
		return unmarshaler.UnmarshalLogs(buf)
	})
}

// flattenAllLogs flattens an array of plog.Logs to a slice of FlatLogOtel.
// It converts the deeply nested plog.Logs data structure to a flat struct, to make it more readable in the test output logs.
func flattenAllLogs(lds []plog.Logs) []FlatLog {
	var flatLogs []FlatLog

	for _, ld := range lds {
		flatLogs = append(flatLogs, flattenLogs(ld)...)
	}

	return flatLogs
}

// flattenLogs converts a single plog.Logs to a slice of FlatLogOtel
// It takes relevant information from different levels of pdata and puts it into a FlatLogOtel go struct.
func flattenLogs(ld plog.Logs) []FlatLog {
	var flatLogs []FlatLog

	for i := range ld.ResourceLogs().Len() {
		resourceLogs := ld.ResourceLogs().At(i)
		for j := range resourceLogs.ScopeLogs().Len() {
			scopeLogs := resourceLogs.ScopeLogs().At(j)
			for k := range scopeLogs.LogRecords().Len() {
				lr := scopeLogs.LogRecords().At(k)
				flatLogs = append(flatLogs, FlatLog{
					ResourceAttributes: attributesToMap(resourceLogs.Resource().Attributes()),
					ScopeName:          scopeLogs.Scope().Name(),
					ScopeVersion:       scopeLogs.Scope().Version(),
					LogRecordBody:      lr.Body().AsString(),
					Attributes:         attributesToMap(lr.Attributes()),
					ObservedTimestamp:  lr.ObservedTimestamp().String(),
					Timestamp:          lr.Timestamp().String(),
					TraceId:            lr.TraceID().String(),
					SpanId:             lr.SpanID().String(),
					TraceFlags:         uint32(lr.Flags()),
					SeverityText:       lr.SeverityText(),
					SeverityNumber:     int(lr.SeverityNumber()),
				})
			}
		}
	}

	return flatLogs
}

// attributesToMap converts pdata.AttributeMap to a map using the string representation of the values.
func attributesToMap(attrs pcommon.Map) map[string]string {
	attrMap := make(map[string]string)

	attrs.Range(func(k string, v pcommon.Value) bool {
		attrMap[k] = v.AsString()
		return true
	})

	return attrMap
}
