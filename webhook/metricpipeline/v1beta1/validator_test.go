package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

// Note: Most validation behavior is tested in webhook/common/validator_test.go
// These tests focus on MetricPipeline-specific behavior

func TestMetricPipelineValidator_ValidateCreate(t *testing.T) {
	validator := NewMetricPipelineValidator()

	pipeline := &telemetryv1beta1.MetricPipeline{
		Spec: telemetryv1beta1.MetricPipelineSpec{
			Filters: []telemetryv1beta1.FilterSpec{
				{
					Conditions: []string{`IsMatch(metric.name, "envoy") == true`},
				},
			},
		},
	}

	_, err := validator.ValidateCreate(t.Context(), pipeline)
	require.NoError(t, err)
}

func TestMetricPipelineValidator_ValidateUpdate(t *testing.T) {
	validator := NewMetricPipelineValidator()

	oldPipeline := &telemetryv1beta1.MetricPipeline{}
	newPipeline := &telemetryv1beta1.MetricPipeline{
		Spec: telemetryv1beta1.MetricPipelineSpec{
			Filters: []telemetryv1beta1.FilterSpec{
				{
					Conditions: []string{`IsMatch(metric.name, "envoy") == true`},
				},
			},
		},
	}

	_, err := validator.ValidateUpdate(t.Context(), oldPipeline, newPipeline)
	require.NoError(t, err)
}

func TestMetricPipelineValidator_ValidateDelete(t *testing.T) {
	validator := NewMetricPipelineValidator()
	pipeline := &telemetryv1beta1.MetricPipeline{}

	warnings, err := validator.ValidateDelete(t.Context(), pipeline)

	require.NoError(t, err)
	require.Empty(t, warnings)
}

func TestMetricPipelineValidator_WrongType(t *testing.T) {
	validator := NewMetricPipelineValidator()
	wrongObject := &telemetryv1beta1.TracePipeline{}

	warnings, err := validator.ValidateCreate(t.Context(), wrongObject)

	assert.ErrorContains(t, err, "expected a *v1beta1.MetricPipeline but got")
	assert.Empty(t, warnings)
}
