package featureflags

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFeatureFlags(t *testing.T) {
	assert.False(t, Isv1Beta1Enabled())
	Setv1Beta1Enabled(true)
	assert.True(t, Isv1Beta1Enabled())

	assert.False(t, IsLogPipelineOTLPEnabled())
	SetLogPipelineOTLPEnabled(true)
	assert.True(t, IsLogPipelineOTLPEnabled())
}
