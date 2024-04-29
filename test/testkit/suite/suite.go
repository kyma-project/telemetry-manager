package suite

import (
	"path"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
)

func Current() string {
	_, filePath, _, ok := runtime.Caller(1)
	if !ok {
		panic("Cannot get the current file path")
	}

	fileName := path.Base(filePath)
	specID := strings.TrimSuffix(fileName, "_test.go")
	specID = strings.ReplaceAll(specID, "_", "-")

	return specID
}

const (
	LabelLogs                  = "logs"
	LabelTraces                = "traces"
	LabelMetrics               = "metrics"
	LabelTelemetry             = "telemetry"
	LabelSelfMonitoringLogs    = "self-mon-logs"
	LabelSelfMonitoringTraces  = "self-mon-traces"
	LabelSelfMonitoringMetrics = "self-mon-metrics"
	LabelV1Beta1               = "v1beta1"
	LabelTelemetryLogsAnalysis = "telemetry-logs-analysis"

	// Operational tests preserve K8s objects between test runs.
	LabelOperational = "operational"
)

// IsOperational returns true if the test is invoked with an "operational" tag.
func IsOperational() bool {
	labelsFilter := GinkgoLabelFilter()

	return labelsFilter != "" && Label(LabelOperational).MatchesLabelFilter(labelsFilter)
}
