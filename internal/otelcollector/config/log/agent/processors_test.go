package agent

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestProcessorConfig(t *testing.T) {
	processorsConfig := makeProcessorsConfig("v1.0.0")
	require.Equal(t, "scope", processorsConfig.SetInstrumentationScopeRuntime.Context)
	require.Len(t, processorsConfig.SetInstrumentationScopeRuntime.Statements, 2)
	require.Equal(t, "set(version, \"v1.0.0\")", processorsConfig.SetInstrumentationScopeRuntime.Statements[0])
	require.Equal(t, "set(name, \"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver\")", processorsConfig.SetInstrumentationScopeRuntime.Statements[1])

	require.Equal(t, "5s", processorsConfig.BaseProcessors.MemoryLimiter.CheckInterval)
	require.Equal(t, 80, processorsConfig.BaseProcessors.MemoryLimiter.LimitPercentage)
	require.Equal(t, 25, processorsConfig.BaseProcessors.MemoryLimiter.SpikeLimitPercentage)

}
