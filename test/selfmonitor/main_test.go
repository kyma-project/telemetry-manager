package selfmonitor

import (
	"log"
	"os"
	"testing"

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

// labelsForSelfMonitor returns the appropriate labels for a selfmonitor test.
// It includes:
// - The combined selfmonitor label (e.g., "selfmonitor-log-agent-healthy")
// - istio label for backpressure/outage scenarios (they need Istio for traffic simulation)
// - no-fips label when noFips is true
func labelsForSelfMonitor(selfMonitorLabelPrefix, selfMonitorLabelSuffix string, noFips bool) []string {
	// Build the combined label (e.g., "selfmonitor-log-agent-healthy")
	combinedLabel := selfMonitorLabelPrefix + "-" + selfMonitorLabelSuffix

	labels := []string{combinedLabel}

	// Backpressure and outage tests need Istio for traffic simulation
	if selfMonitorLabelSuffix == suite.LabelBackpressure ||
		selfMonitorLabelSuffix == suite.LabelOutage {
		labels = append(labels, suite.LabelIstio)
	}

	if noFips {
		labels = append(labels, suite.LabelNoFIPS)
	}

	return labels
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
