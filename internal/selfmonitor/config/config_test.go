package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	config := SelfMonitor{
		BaseName:  "telemetry",
		Namespace: "kyma",
	}
	monitoringConfgYaml := "foo-bar"
	selfMonitorConfig := config.WithMonitoringConfig(monitoringConfgYaml)
	require.Equal(t, selfMonitorConfig.MonitoringConfig, monitoringConfgYaml)
}
