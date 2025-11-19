package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
)

func TestValidateFilterTransform_Success(t *testing.T) {
	err := ValidateFilterTransform(
		ottl.SignalTypeLog,
		[]telemetryv1alpha1.FilterSpec{},
		[]telemetryv1alpha1.TransformSpec{},
	)
	assert.NoError(t, err)
}

func TestValidateFilterTransform_ReturnsErrorOnInvalidInput(t *testing.T) {
	err := ValidateFilterTransform(
		ottl.SignalType("invalid"),
		[]telemetryv1alpha1.FilterSpec{},
		[]telemetryv1alpha1.TransformSpec{},
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to instantiate")
}
func TestValidateFilterTransformV1Beta1_Success(t *testing.T) {
	err := ValidateFilterTransformV1Beta1(
		ottl.SignalTypeLog,
		[]telemetryv1beta1.FilterSpec{},
		[]telemetryv1beta1.TransformSpec{},
	)
	assert.NoError(t, err)
}

func TestValidateFilterTransformV1Beta1_ReturnsErrorOnInvalidInput(t *testing.T) {
	err := ValidateFilterTransformV1Beta1(
		ottl.SignalType("invalid"),
		[]telemetryv1beta1.FilterSpec{},
		[]telemetryv1beta1.TransformSpec{},
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to instantiate")
}
