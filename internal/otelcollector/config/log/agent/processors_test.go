package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProcessorConfig(t *testing.T) {
	processorsConfig := makeProcessorsConfig("v1.0.0")
	require.Equal(t, "scope", processorsConfig.SetInstrumentationScopeRuntime.LogStatements[0].Context)
	require.Len(t, processorsConfig.SetInstrumentationScopeRuntime.LogStatements[0].Statements, 2)
	require.Equal(t, "set(version, \"v1.0.0\")", processorsConfig.SetInstrumentationScopeRuntime.LogStatements[0].Statements[0])
	require.Equal(t, "set(name, \"io.kyma-project.telemetry/runtime\")", processorsConfig.SetInstrumentationScopeRuntime.LogStatements[0].Statements[1])

	require.Equal(t, "5s", processorsConfig.BaseProcessors.MemoryLimiter.CheckInterval)
	require.Equal(t, 80, processorsConfig.BaseProcessors.MemoryLimiter.LimitPercentage)
	require.Equal(t, 25, processorsConfig.BaseProcessors.MemoryLimiter.SpikeLimitPercentage)
}
