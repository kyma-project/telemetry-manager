package suite

import (
	"path"
	"runtime"
	"strings"
)

func Current() string {
	_, filePath, _, _ := runtime.Caller(1)
	fileName := path.Base(filePath)
	specID := strings.TrimSuffix(fileName, "_test.go")
	specID = strings.ReplaceAll(specID, "_", "-")

	return specID
}

const (
	LabelLogs      = "logs"
	LabelTraces    = "traces"
	LabelMetrics   = "metrics"
	LabelTelemetry = "telemetry"

	LabelSelfMonitoringLogs = "self-mon-logs"
)
