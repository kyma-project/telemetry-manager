package suite

import (
	"context"
	"path"
	"runtime"
	"strings"

	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
	. "github.com/onsi/ginkgo/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	Ctx                context.Context
	Cancel             context.CancelFunc
	K8sClient          client.Client
	ProxyClient        *apiserverproxy.Client
	TestEnv            *envtest.Environment
	TelemetryK8sObject client.Object
	K8sObjects         []client.Object
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
	LabelLogs                 = "logs"
	LabelTraces               = "traces"
	LabelMetrics              = "metrics"
	LabelTelemetry            = "telemetry"
	LabelExperimental         = "experimental"
	LabelTelemetryLogAnalysis = "telemetry-log-analysis"
	LabelMaxPipeline          = "max-pipeline"
	LabelSetA                 = "set_a"
	LabelSetB                 = "set_b"
	LabelSetC                 = "set_c"

	LabelSelfMonitoringLogsHealthy         = "self-mon-logs-healthy"
	LabelSelfMonitoringLogsBackpressure    = "self-mon-logs-backpressure"
	LabelSelfMonitoringLogsOutage          = "self-mon-logs-outage"
	LabelSelfMonitoringTracesHealthy       = "self-mon-traces-healthy"
	LabelSelfMonitoringTracesBackpressure  = "self-mon-traces-backpressure"
	LabelSelfMonitoringTracesOutage        = "self-mon-traces-outage"
	LabelSelfMonitoringMetricsHealthy      = "self-mon-metrics-healthy"
	LabelSelfMonitoringMetricsBackpressure = "self-mon-metrics-backpressure"
	LabelSelfMonitoringMetricsOutage       = "self-mon-metrics-outage"

	// Istio test label
	LabelIntegration = "integration"

	// Upgrade tests preserve K8s objects between test runs.
	LabelUpgrade = "upgrade"
)

// IsUpgrade returns true if the test is invoked with an "upgrade" tag.
func IsUpgrade() bool {
	labelsFilter := GinkgoLabelFilter()

	return labelsFilter != "" && Label(LabelUpgrade).MatchesLabelFilter(labelsFilter)
}
