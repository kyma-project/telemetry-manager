package selfmonitor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	config := Config{
		BaseName:  "telemetry",
		Namespace: "kyma",
	}
	monitoringConfgYaml := "foo-bar"
	selfMonitorConfig := config.WithMonitoringConfig(monitoringConfgYaml)
	require.Equal(t, selfMonitorConfig.monitoringConfig, monitoringConfgYaml)
}
