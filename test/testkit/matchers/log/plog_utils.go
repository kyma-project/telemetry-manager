package log

import (
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func unmarshalLogs(jsonlMetrics []byte) ([]plog.Logs, error) {
	return matchers.UnmarshalSignals[plog.Logs](jsonlMetrics, func(buf []byte) (plog.Logs, error) {
		var unmarshaler plog.JSONUnmarshaler
		return unmarshaler.UnmarshalLogs(buf)
	})
}

func getLogRecords(ld plog.Logs) []plog.LogRecord {
	var logRecords []plog.LogRecord

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		resourceLogs := ld.ResourceLogs().At(i)
		for j := 0; j < resourceLogs.ScopeLogs().Len(); j++ {
			scopeLogs := resourceLogs.ScopeLogs().At(j)
			for k := 0; k < scopeLogs.LogRecords().Len(); k++ {
				logRecords = append(logRecords, scopeLogs.LogRecords().At(k))
			}
		}
	}

	return logRecords
}

func getKubernetesAttributes(lr plog.LogRecord) map[string]any {
	const kubernetesAttrKey = "kubernetes"
	kubernetesAttrs, hasKubernetesAttrs := lr.Attributes().Get(kubernetesAttrKey)
	if !hasKubernetesAttrs || kubernetesAttrs.Type() != pcommon.ValueTypeMap {
		return nil
	}
	return kubernetesAttrs.Map().AsRaw()
}
