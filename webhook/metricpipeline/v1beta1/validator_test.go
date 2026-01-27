package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/assert"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

func TestMetricPipelineValidator_ValidateCreate(t *testing.T) {
	tests := []struct {
		name      string
		pipeline  *telemetryv1beta1.MetricPipeline
		expectErr bool
	}{
		{
			name: "valid filter",
			pipeline: &telemetryv1beta1.MetricPipeline{
				Spec: telemetryv1beta1.MetricPipelineSpec{
					Filters: []telemetryv1beta1.FilterSpec{
						{
							Conditions: []string{`IsMatch(metric.name, "envoy") == true`},
						},
					},
					Transforms: []telemetryv1beta1.TransformSpec{},
				},
			},
			expectErr: false,
		},
		{
			name: "invalid filter - bad OTTL expression",
			pipeline: &telemetryv1beta1.MetricPipeline{
				Spec: telemetryv1beta1.MetricPipelineSpec{
					Filters: []telemetryv1beta1.FilterSpec{
						{
							Conditions: []string{`IsMatch(metric.name, "envoy") ?= true`},
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "empty fields - should pass",
			pipeline: &telemetryv1beta1.MetricPipeline{
				Spec: telemetryv1beta1.MetricPipelineSpec{
					Filters:    []telemetryv1beta1.FilterSpec{},
					Transforms: []telemetryv1beta1.TransformSpec{},
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &validator{}

			_, err := validator.ValidateCreate(t.Context(), tt.pipeline)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMetricPipelineValidator_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name        string
		oldPipeline *telemetryv1beta1.MetricPipeline
		newPipeline *telemetryv1beta1.MetricPipeline
		expectErr   bool
	}{
		{
			name: "valid update",
			oldPipeline: &telemetryv1beta1.MetricPipeline{
				Spec: telemetryv1beta1.MetricPipelineSpec{
					Filters: []telemetryv1beta1.FilterSpec{},
				},
			},
			newPipeline: &telemetryv1beta1.MetricPipeline{
				Spec: telemetryv1beta1.MetricPipelineSpec{
					Filters: []telemetryv1beta1.FilterSpec{
						{
							Conditions: []string{`IsMatch(metric.name, "envoy") == true`},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "invalid update - bad filter",
			oldPipeline: &telemetryv1beta1.MetricPipeline{
				Spec: telemetryv1beta1.MetricPipelineSpec{
					Filters: []telemetryv1beta1.FilterSpec{},
				},
			},
			newPipeline: &telemetryv1beta1.MetricPipeline{
				Spec: telemetryv1beta1.MetricPipelineSpec{
					Filters: []telemetryv1beta1.FilterSpec{
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
			validator := &validator{}

			_, err := validator.ValidateUpdate(t.Context(), tt.oldPipeline, tt.newPipeline)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMetricPipelineValidator_ValidateDelete(t *testing.T) {
	validator := &validator{}

	pipeline := &telemetryv1beta1.MetricPipeline{}

	warnings, err := validator.ValidateDelete(t.Context(), pipeline)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}
