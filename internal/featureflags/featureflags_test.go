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
}

func TestEnabledFlags(t *testing.T) {
	Enable(V1Beta1)

	flags := EnabledFlags()
	assert.Len(t, flags, 1)
	assert.Contains(t, flags, V1Beta1)

	Disable(V1Beta1)

	flags = EnabledFlags()
	assert.Len(t, flags, 0)
}

func TestFeatureFlag_String(t *testing.T) {
	assert.Equal(t, "V1Beta1", V1Beta1.String())
	assert.Equal(t, "FeatureFlag(2)", FeatureFlag(2).String())
}
