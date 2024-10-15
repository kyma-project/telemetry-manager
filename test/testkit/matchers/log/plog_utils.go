package log

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/kyma-project/telemetry-manager/test/testkit/matchers"
)

// FlatLog holds all needed information about a log record.
// Gomega doesn't handle deeply nested data structure very well and generates large, unreadable diffs when paired with the deeply nested structure of plogs.
//
// Introducing a go struct with a flat data structure by extracting necessary information from different levels of plogs makes accessing the information easier than using plog.Logs directly and improves the readability of the test output logs.
type FlatLog struct {
	LogRecordAttributes            map[string]string
	LogRecordBody                  string
	Timestamp                      time.Time
	Level                          string
	PodName                        string
	ContainerName                  string
	NamespaceName                  string
	KubernetesLabelAttributes      map[string]any
	KubernetesAnnotationAttributes map[string]any
}

func unmarshalLogs(jsonlMetrics []byte) ([]plog.Logs, error) {
	return matchers.UnmarshalSignals[plog.Logs](jsonlMetrics, func(buf []byte) (plog.Logs, error) {
		var unmarshaler plog.JSONUnmarshaler
		return unmarshaler.UnmarshalLogs(buf)
	})
}

// flattenAllLogs flattens an array of pdata.Logs log record to a slice of FlatLog.
// It converts the deeply nested pdata.Logs data structure to a flat struct, to make it more readable in the test output logs.
func flattenAllLogs(lds []plog.Logs) []FlatLog {
	var flatLogs []FlatLog

	for _, ld := range lds {
		flatLogs = append(flatLogs, flattenLogs(ld)...)
	}

	return flatLogs
}

// flattenMetrics converts a single pdata.Log log record to a slice of FlatMetric
// It takes relevant information from different levels of pdata and puts it into a FlatLog go struct.
func flattenLogs(ld plog.Logs) []FlatLog {
	var flatLogs []FlatLog

	for i := range ld.ResourceLogs().Len() {
		resourceLogs := ld.ResourceLogs().At(i)
		for j := range resourceLogs.ScopeLogs().Len() {
			scopeLogs := resourceLogs.ScopeLogs().At(j)
			for k := range scopeLogs.LogRecords().Len() {
				lr := scopeLogs.LogRecords().At(k)
				k8sAttrs := getKubernetesAttributes(lr)

				flatLogs = append(flatLogs, FlatLog{
					LogRecordAttributes:            attributeToMap(lr.Attributes()),
					LogRecordBody:                  lr.Body().AsString(),
					Timestamp:                      parseTimestamp(getAttribute("timestamp", lr.Attributes())),
					Level:                          getAttribute("level", lr.Attributes()),
					PodName:                        getAttribute("pod_name", k8sAttrs),
					ContainerName:                  getAttribute("container_name", k8sAttrs),
					NamespaceName:                  getAttribute("namespace_name", k8sAttrs),
					KubernetesLabelAttributes:      mapKubernetesAttributes("labels", k8sAttrs),
					KubernetesAnnotationAttributes: mapKubernetesAttributes("annotations", k8sAttrs),
				})
			}
		}
	}

	return flatLogs
}

// attributeToMap converts pdata.AttributeMap to a map using the string representation of the values.
func attributeToMap(attrs pcommon.Map) map[string]string {
	attrMap := make(map[string]string)

	attrs.Range(func(k string, v pcommon.Value) bool {
		// only take if value is not of type map, to reduce nesting and avoid duplication of kubernetes attributes
		if v.Type() == pcommon.ValueTypeMap {
			return false
		}

		attrMap[k] = v.AsString()

		return true
	})

	return attrMap
}

// mapKubernetesAttributes converts the kubernetes attributes from a LogRecord which are of type
// ValueTypeMap into a map using the string representation of the keys and any representation of the values
func mapKubernetesAttributes(key string, attrs pcommon.Map) map[string]any {
	attr, hasAttr := attrs.Get(key)
	if !hasAttr || attr.Type() != pcommon.ValueTypeMap {
		return nil
	}

	return attr.Map().AsRaw()
}

// getAttribute takes an input key and returns the map value associated with the key if it exists, and returns
// an empty string if it doesn't
func getAttribute(key string, p pcommon.Map) string {
	attr, hasAttr := p.Get(key)
	if !hasAttr || attr.Type() != pcommon.ValueTypeStr {
		return ""
	}

	return attr.Str()
}

func parseTimestamp(ts string) time.Time {
	timestamp, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return time.Time{}
	}

	return timestamp
}

func getKubernetesAttributes(lr plog.LogRecord) pcommon.Map {
	const kubernetesAttrKey = "kubernetes"

	kubernetesAttrs, hasKubernetesAttrs := lr.Attributes().Get(kubernetesAttrKey)
	if !hasKubernetesAttrs || kubernetesAttrs.Type() != pcommon.ValueTypeMap {
		return pcommon.NewMap()
	}

	return kubernetesAttrs.Map()
}
