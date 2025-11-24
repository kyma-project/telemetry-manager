package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func TestTracePipelineValidator_ValidateCreate(t *testing.T) {
	tests := []struct {
		name      string
		pipeline  *telemetryv1alpha1.TracePipeline
		expectErr bool
	}{
		{
			name: "valid filter",
			pipeline: &telemetryv1alpha1.TracePipeline{
				Spec: telemetryv1alpha1.TracePipelineSpec{
					Filters: []telemetryv1alpha1.FilterSpec{
						{
							Conditions: []string{`IsMatch(span.attributes, "envoy") == true`},
						},
					},
					Transforms: []telemetryv1alpha1.TransformSpec{},
				},
			},
			expectErr: false,
		},
		{
			name: "invalid filter - bad OTTL expression",
			pipeline: &telemetryv1alpha1.TracePipeline{
				Spec: telemetryv1alpha1.TracePipelineSpec{
					Filters: []telemetryv1alpha1.FilterSpec{
						{
							Conditions: []string{`IsMatch(span.attributes, "envoy") ?= true`},
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "empty fields - should pass",
			pipeline: &telemetryv1alpha1.TracePipeline{
				Spec: telemetryv1alpha1.TracePipelineSpec{
					Filters:    []telemetryv1alpha1.FilterSpec{},
					Transforms: []telemetryv1alpha1.TransformSpec{},
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &TracePipelineValidator{}

			_, err := validator.ValidateCreate(t.Context(), tt.pipeline)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTracePipelineValidator_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name        string
		oldPipeline *telemetryv1alpha1.TracePipeline
		newPipeline *telemetryv1alpha1.TracePipeline
		expectErr   bool
	}{
		{
			name: "valid update",
			oldPipeline: &telemetryv1alpha1.TracePipeline{
				Spec: telemetryv1alpha1.TracePipelineSpec{
					Filters: []telemetryv1alpha1.FilterSpec{},
				},
			},
			newPipeline: &telemetryv1alpha1.TracePipeline{
				Spec: telemetryv1alpha1.TracePipelineSpec{
					Filters: []telemetryv1alpha1.FilterSpec{
						{
							Conditions: []string{`IsMatch(span.attributes, "envoy") == true`},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "invalid update - bad filter",
			oldPipeline: &telemetryv1alpha1.TracePipeline{
				Spec: telemetryv1alpha1.TracePipelineSpec{
					Filters: []telemetryv1alpha1.FilterSpec{},
				},
			},
			newPipeline: &telemetryv1alpha1.TracePipeline{
				Spec: telemetryv1alpha1.TracePipelineSpec{
					Filters: []telemetryv1alpha1.FilterSpec{
						{
							Conditions: []string{`log.severity_number <? SEVERITY_NUMBER_WARN`},
						},
					},
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &TracePipelineValidator{}

			_, err := validator.ValidateUpdate(t.Context(), tt.oldPipeline, tt.newPipeline)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTracePipelineValidator_ValidateDelete(t *testing.T) {
	validator := &TracePipelineValidator{}

	pipeline := &telemetryv1alpha1.LogPipeline{}

	warnings, err := validator.ValidateDelete(t.Context(), pipeline)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestTracePipelineValidator_WrongType(t *testing.T) {
	validator := &TracePipelineValidator{}

	// Pass wrong type
	wrongObject := &telemetryv1alpha1.LogPipeline{}

	warnings, err := validator.ValidateCreate(t.Context(), wrongObject)

	assert.ErrorContains(t, err, "expected a TracePipeline but got")
	assert.Empty(t, warnings)
}
