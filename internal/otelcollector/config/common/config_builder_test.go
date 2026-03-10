package common

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

func TestNewConfig(t *testing.T) {
	t.Run("creates config with all required components", func(t *testing.T) {
		config := NewConfig()

		require.NotNil(t, config, "config should not be nil")
		require.NotNil(t, config.Receivers, "receivers map should be initialized")
		require.NotNil(t, config.Processors, "processors map should be initialized")
		require.NotNil(t, config.Exporters, "exporters map should be initialized")
		require.NotNil(t, config.Connectors, "connectors map should be initialized")
		require.NotNil(t, config.Extensions, "extensions map should be initialized")
		require.NotNil(t, config.Service, "service config should be initialized")

		require.Empty(t, config.Receivers, "receivers map should be empty initially")
		require.Empty(t, config.Processors, "processors map should be empty initially")
		require.Empty(t, config.Exporters, "exporters map should be empty initially")
		require.Empty(t, config.Connectors, "connectors map should be empty initially")

		require.Len(t, config.Extensions, 3, "should have exactly 3 extensions configured")
		require.Contains(t, config.Extensions, ComponentIDHealthCheckExtension, "health check extension should be configured")
		require.Contains(t, config.Extensions, ComponentIDPprofExtension, "pprof extension should be configured")
		require.Contains(t, config.Extensions, ComponentIDCGroupRuntimeExtension, "cgroupruntime extension should be configured")

		require.NotNil(t, config.Service.Pipelines, "service pipelines should be initialized")
		require.Empty(t, config.Service.Pipelines, "service pipelines should be empty initially")
		require.Len(t, config.Service.Extensions, 3, "service should reference exactly 3 extensions")
		require.Contains(t, config.Service.Extensions, ComponentIDHealthCheckExtension, "service should reference health check extension")
		require.Contains(t, config.Service.Extensions, ComponentIDPprofExtension, "service should reference pprof extension")
		require.Contains(t, config.Service.Extensions, ComponentIDCGroupRuntimeExtension, "service should reference cgroupruntime extension")
	})
}

func TestServiceConfig(t *testing.T) {
	t.Run("creates service config with telemetry setup", func(t *testing.T) {
		service := service()

		require.NotNil(t, service.Pipelines, "pipelines should be initialized")
		require.Empty(t, service.Pipelines, "pipelines should be empty initially")
		require.Len(t, service.Extensions, 3, "should reference exactly 3 extensions")
		require.Contains(t, service.Extensions, ComponentIDHealthCheckExtension, "should reference health check extension")
		require.Contains(t, service.Extensions, ComponentIDPprofExtension, "should reference pprof extension")
		require.Contains(t, service.Extensions, ComponentIDCGroupRuntimeExtension, "should reference cgroupruntime extension")

		require.NotNil(t, service.Telemetry, "telemetry config should be initialized")

		require.NotNil(t, service.Telemetry.Metrics, "metrics config should be initialized")
		require.Len(t, service.Telemetry.Metrics.Readers, 1, "should have exactly 1 metric reader")

		reader := service.Telemetry.Metrics.Readers[0]
		require.NotNil(t, reader.Pull, "pull metric reader should be configured")
		require.NotNil(t, reader.Pull.Exporter, "metric exporter should be configured")
		require.NotNil(t, reader.Pull.Exporter.Prometheus, "prometheus exporter should be configured")

		prometheus := reader.Pull.Exporter.Prometheus
		require.Equal(t, "${MY_POD_IP}", prometheus.Host, "prometheus should use MY_POD_IP environment variable")
		require.Equal(t, ports.Metrics, prometheus.Port, "prometheus should use correct metrics port")

		require.NotNil(t, service.Telemetry.Logs, "logs config should be initialized")
		require.Equal(t, "info", service.Telemetry.Logs.Level, "logs should use info level by default")
		require.Equal(t, "json", service.Telemetry.Logs.Encoding, "logs should use json encoding by default")
	})
}

func TestExtensionsConfig(t *testing.T) {
	t.Run("creates extensions config with health check, pprof, and cgroupruntime", func(t *testing.T) {
		extensions := extensions()

		require.Len(t, extensions, 3, "should configure exactly 3 extensions")
		require.Contains(t, extensions, ComponentIDHealthCheckExtension, "health check extension should be present")
		require.Contains(t, extensions, ComponentIDPprofExtension, "pprof extension should be present")
		require.Contains(t, extensions, ComponentIDCGroupRuntimeExtension, "cgroupruntime extension should be present")

		healthCheck, ok := extensions[ComponentIDHealthCheckExtension]
		require.True(t, ok, "health check extension should exist")

		healthCheckEndpoint, ok := healthCheck.(Endpoint)
		require.True(t, ok, "health check extension should be an Endpoint struct")
		require.Equal(t, "${MY_POD_IP}:13133", healthCheckEndpoint.Endpoint, "health check should use MY_POD_IP and correct port")

		pprof, ok := extensions[ComponentIDPprofExtension]
		require.True(t, ok, "pprof extension should exist")

		pprofEndpoint, ok := pprof.(Endpoint)
		require.True(t, ok, "pprof extension should be an Endpoint struct")
		require.Equal(t, "127.0.0.1:1777", pprofEndpoint.Endpoint, "pprof should use localhost and correct port")

		cgroupruntime, ok := extensions[ComponentIDCGroupRuntimeExtension]
		require.True(t, ok, "cgroupruntime extension should exist")

		cgroupruntimeExt, ok := cgroupruntime.(CGroupRuntimeExtension)
		require.True(t, ok, "cgroupruntime extension should be a CGroupRuntimeExtension struct")
		require.False(t, cgroupruntimeExt.GoMaxProcs.Enabled, "gomaxprocs should be not enabled")
		require.True(t, cgroupruntimeExt.GoMemLimit.Enabled, "gomemlimit should be enabled")
		require.InDelta(t, 0.8, cgroupruntimeExt.GoMemLimit.Ratio, 0.001, "gomemlimit ratio should be 0.8")
	})
}
