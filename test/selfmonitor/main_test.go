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
	const errorCode = 1

	if err := suite.BeforeSuiteFunc(); err != nil {
		log.Printf("Setup failed: %v", err)
		os.Exit(errorCode)
	}

	m.Run()
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
