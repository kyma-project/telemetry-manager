package log

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/kyma-project/telemetry-manager/test/testkit/matchers"
)

type FlatLog struct {
	LogRecordAttributes            map[string]string
	Timestamp                      time.Time
	LogRecordBody                  string
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

func flattenAllLogs(lds []plog.Logs) []FlatLog {
	var flatLogs []FlatLog

	for _, ld := range lds {
		flatLogs = append(flatLogs, flattenLogs(ld)...)
	}
	return flatLogs
}

func flattenLogs(ld plog.Logs) []FlatLog {
	var flatLogs []FlatLog

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		resourceLogs := ld.ResourceLogs().At(i)
		for j := 0; j < resourceLogs.ScopeLogs().Len(); j++ {
			scopeLogs := resourceLogs.ScopeLogs().At(j)
			for k := 0; k < scopeLogs.LogRecords().Len(); k++ {
				lr := scopeLogs.LogRecords().At(k)
				k8sAttrs := getKubernetesAttributes(lr)

				flatLogs = append(flatLogs, FlatLog{
					LogRecordAttributes:            attributeToMap(lr.Attributes()),
					Timestamp:                      getTimestamp(attributeToMap(lr.Attributes())),
					LogRecordBody:                  lr.Body().AsString(),
					Level:                          getAttribute("level", lr.Attributes()),
					PodName:                        getAttribute("pod_name", k8sAttrs),
					ContainerName:                  getAttribute("container_name", k8sAttrs),
					NamespaceName:                  getAttribute("namespace_name", k8sAttrs),
					KubernetesLabelAttributes:      mapKubernetesMapAttributes("labels", k8sAttrs),
					KubernetesAnnotationAttributes: mapKubernetesMapAttributes("annotations", k8sAttrs),
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
		attrMap[k] = v.AsString()
		return true
	})
	return attrMap
}

// mapKubernetesMapAttributes converts the kubernetes attributes from a LogRecord which are of type
// ValueTypeMap into a map using the string representation of the keys and 	map representation of the values
func mapKubernetesMapAttributes(key string, attrs pcommon.Map) map[string]any {
	attr, hasAttr := attrs.Get(key)
	if !hasAttr || attr.Type() != pcommon.ValueTypeMap {
		return nil
	}
	return attr.Map().AsRaw()
}

func getAttribute(name string, p pcommon.Map) string {
	attr, hasAttr := p.Get(name)
	if !hasAttr || attr.Type() != pcommon.ValueTypeStr {
		return ""
	}
	return attr.Str()
}

func getTimestamp(lr map[string]string) time.Time {
	ts, ok := lr["timestamp"]
	if !ok {
		return time.Time{}
	}
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
