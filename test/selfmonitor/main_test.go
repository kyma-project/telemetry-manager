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

// signalTypeForComponent returns the backend signal type for a component label
func signalTypeForComponent(component string) kitbackend.SignalType {
	switch component {
	case suite.LabelLogAgent, suite.LabelLogGateway:
		return kitbackend.SignalTypeLogsOTel
	case suite.LabelFluentBit:
		return kitbackend.SignalTypeLogsFluentBit
	case suite.LabelMetricAgent, suite.LabelMetricGateway:
		return kitbackend.SignalTypeMetrics
	case suite.LabelTraces:
		return kitbackend.SignalTypeTraces
	default:
		return ""
	}
}

// isFluentBit returns true if the component is FluentBit (which doesn't support FIPS mode)
func isFluentBit(component string) bool {
	return component == suite.LabelFluentBit
}
