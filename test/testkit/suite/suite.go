package suite

import (
	"path"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
)

// ID returns the current test suite ID.
// It is based on the file name of the test suite.
// It is useful for generating unique names for resources created in the test suite (telemetry pipelines, mock namespaces, etc.).
func ID() string {
	_, filePath, _, ok := runtime.Caller(1)
	if !ok {
		panic("Cannot get the current file path")
	}

	return sanitizeSpecID(filePath)
}

// IDWithSuffix returns the current test suite ID with the provided suffix.
// If no suffix is provided, it defaults to an empty string.
func IDWithSuffix(suffix string) string {
	_, filePath, _, ok := runtime.Caller(1)
	if !ok {
		panic("Cannot get the current file path")
	}

	return sanitizeSpecID(filePath) + "-" + suffix
}

func sanitizeSpecID(filePath string) string {
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
