package featureflags

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFeatureFlags(t *testing.T) {
	assert.False(t, IsEnabled(placeholder))
	Enable(placeholder)
	assert.True(t, IsEnabled(placeholder))
	Disable(placeholder)
	assert.False(t, IsEnabled(placeholder))
}

func TestEnabledFlags(t *testing.T) {
	Enable(placeholder)

	flags := EnabledFlags()
	assert.Len(t, flags, 1)
	assert.Contains(t, flags, placeholder)

	Disable(placeholder)

	flags = EnabledFlags()
	assert.Len(t, flags, 0)
}

func TestFeatureFlag_String(t *testing.T) {
	assert.Equal(t, "placeholder", placeholder.String())
	assert.Equal(t, "FeatureFlag(2)", FeatureFlag(2).String())
}
