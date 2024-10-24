package featureflags

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFeatureFlags(t *testing.T) {
	assert.False(t, IsEnabled(V1Beta1))
	Enable(V1Beta1)
	assert.True(t, IsEnabled(V1Beta1))
	Disable(V1Beta1)
	assert.False(t, IsEnabled(V1Beta1))

	assert.False(t, IsEnabled(LogPipelineOTLP))
	Enable(LogPipelineOTLP)
	assert.True(t, IsEnabled(LogPipelineOTLP))
	Disable(LogPipelineOTLP)
	assert.False(t, IsEnabled(LogPipelineOTLP))
}

func TestEnabledFlags(t *testing.T) {
	Enable(V1Beta1)
	Enable(LogPipelineOTLP)

	flags := EnabledFlags()
	assert.Len(t, flags, 2)
	assert.Contains(t, flags, V1Beta1)
	assert.Contains(t, flags, LogPipelineOTLP)

	Disable(V1Beta1)

	flags = EnabledFlags()
	assert.Len(t, flags, 1)
	assert.Contains(t, flags, LogPipelineOTLP)
}
