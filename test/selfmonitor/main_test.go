package selfmonitor

import (
	"fmt"
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

func label(selfMonitorLabelPrefix, selfMonitorLabelSuffix string) string {
	return fmt.Sprintf("%s-%s", selfMonitorLabelPrefix, selfMonitorLabelSuffix)
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
