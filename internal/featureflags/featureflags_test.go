package featureflags

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFeatureFlags(t *testing.T) {
	assert.False(t, IsV1beta1Enabled())
	SetV1beta1Enabled(true)
	assert.True(t, IsV1beta1Enabled())

	assert.False(t, IsLogpipelineOTLPEnabled())
	SetLogpipelineOTLPEnabled(true)
	assert.True(t, IsLogpipelineOTLPEnabled())
}
