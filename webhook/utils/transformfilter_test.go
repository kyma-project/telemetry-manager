package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
)

func TestValidateFilterTransform(t *testing.T) {
	err := ValidateFilterTransform(
		ottl.SignalTypeLog,
		[]telemetryv1beta1.FilterSpec{},
		[]telemetryv1beta1.TransformSpec{},
	)
	assert.NoError(t, err)
}

func TestValidateFilterTransform_(t *testing.T) {
	err := ValidateFilterTransform(
		ottl.SignalType("invalid"),
		[]telemetryv1beta1.FilterSpec{},
		[]telemetryv1beta1.TransformSpec{},
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to instantiate")
}
