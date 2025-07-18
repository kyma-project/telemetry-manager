package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProcessorConfig(t *testing.T) {
	processorsConfig := makeProcessorsConfig(BuildOptions{InstrumentationScopeVersion: "v1.0.0", ClusterName: "cluster", ClusterUID: "cluster-uid", CloudProvider: "provider"})
	require.Len(t, processorsConfig.SetInstrumentationScopeRuntime.LogStatements[0].Statements, 2)
	require.Equal(t, "set(scope.version, \"v1.0.0\")", processorsConfig.SetInstrumentationScopeRuntime.LogStatements[0].Statements[0])
	require.Equal(t, "set(scope.name, \"io.kyma-project.telemetry/runtime\")", processorsConfig.SetInstrumentationScopeRuntime.LogStatements[0].Statements[1])

	require.Equal(t, "5s", processorsConfig.MemoryLimiter.CheckInterval)
	require.Equal(t, 80, processorsConfig.MemoryLimiter.LimitPercentage)
	require.Equal(t, 25, processorsConfig.MemoryLimiter.SpikeLimitPercentage)
}
