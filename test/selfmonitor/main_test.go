package selfmonitor

import (
	"log"
	"os"
	"testing"

	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestMain(m *testing.M) {
	// No explicit cluster configuration or cleanup needed!
	// The dynamic reconfiguration system will:
	// 1. Auto-detect current cluster state (or use defaults if fresh cluster)
	// 2. Reconfigure per-test based on test labels
	// 3. Next test run will detect state and reconfigure as needed - no cleanup required!
	if err := suite.BeforeSuiteFunc(); err != nil {
		log.Printf("Setup failed: %v", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// labelsForSelfMonitor returns the appropriate labels and options for a selfmonitor test.
// It includes:
// - The combined selfmonitor label (e.g., "selfmonitor-log-agent-healthy")
// - kubeprep.WithIstio() option for backpressure/outage scenarios (they need Istio for traffic simulation)
func labelsForSelfMonitor(selfMonitorLabelPrefix, selfMonitorLabelSuffix string) ([]string, []kubeprep.Option) {
	// Build the combined label (e.g., "selfmonitor-log-agent-healthy")
	combinedLabel := selfMonitorLabelPrefix + "-" + selfMonitorLabelSuffix

	labels := []string{combinedLabel}

	var opts []kubeprep.Option

	// Backpressure and outage tests need Istio for traffic simulation
	if selfMonitorLabelSuffix == suite.LabelBackpressure ||
		selfMonitorLabelSuffix == suite.LabelOutage {
		opts = append(opts, kubeprep.WithIstio())
	}

	return labels, opts
}

// isFluentBitTest returns true if the test is for FluentBit (which doesn't support FIPS mode)
func isFluentBitTest(labelPrefix string) bool {
	return labelPrefix == suite.LabelSelfMonitorFluentBitPrefix
}

func signalType(labelPrefix string) kitbackend.SignalType {
	switch labelPrefix {
	case suite.LabelSelfMonitorLogAgentPrefix, suite.LabelSelfMonitorLogGatewayPrefix:
		return kitbackend.SignalTypeLogsOTel
	case suite.LabelSelfMonitorFluentBitPrefix:
		return kitbackend.SignalTypeLogsFluentBit
	case suite.LabelSelfMonitorMetricGatewayPrefix, suite.LabelSelfMonitorMetricAgentPrefix:
		return kitbackend.SignalTypeMetrics
	case suite.LabelSelfMonitorTracesPrefix:
		return kitbackend.SignalTypeTraces
	default:
		return ""
	}
}
