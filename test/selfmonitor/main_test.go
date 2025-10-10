package selfmonitor

import (
	"fmt"
	"log"
	"os"
	"testing"

	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

const (
	kindLogsOTelAgent   string = "logs-otel-agent"
	kindLogsOTelGateway string = "logs-otel-gateway"
	kindLogsFluentbit   string = "logs-fluentbit"
	kindMetricsGateway  string = "metrics-gateway"
	kindMetricsAgent    string = "metrics-agent"
	kindTraces          string = "traces"
)

func TestMain(m *testing.M) {
	const errorCode = 1

	if err := suite.BeforeSuiteFunc(); err != nil {
		log.Printf("Setup failed: %v", err)
		os.Exit(errorCode)
	}

	m.Run()
}

func label(selfmonitorLabel, testKind string) string {
	return fmt.Sprintf("%s-%s", selfmonitorLabel, testKind)
}

func signalType(testKind string) kitbackend.SignalType {
	switch testKind {
	case kindLogsOTelAgent, kindLogsOTelGateway:
		return kitbackend.SignalTypeLogsOTel
	case kindLogsFluentbit:
		return kitbackend.SignalTypeLogsFluentBit
	case kindMetricsGateway, kindMetricsAgent:
		return kitbackend.SignalTypeMetrics
	case kindTraces:
		return kitbackend.SignalTypeTraces
	default:
		return ""
	}
}
