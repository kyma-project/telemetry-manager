package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"k8s.io/utils/ptr"
)

func TestMetricPipelineValidator_ValidateCreate_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name      string
		pipeline  *telemetryv1alpha1.MetricPipeline
		expectErr bool
	}{
		{
			name: "valid ottl filter",
			pipeline: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Filters: []telemetryv1alpha1.FilterSpec{
						{
							Conditions: []string{`IsMatch(metric.name, "envoy") == true`},
						},
					},
					Transforms: []telemetryv1alpha1.TransformSpec{},
				},
			},
			expectErr: false,
		},
		{
			name: "invalid ottl filter - bad OTTL expression",
			pipeline: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Filters: []telemetryv1alpha1.FilterSpec{
						{
							Conditions: []string{`IsMatch(metric.name, "envoy") ?= true`},
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "empty ottl filters and transforms - should pass",
			pipeline: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Filters:    []telemetryv1alpha1.FilterSpec{},
					Transforms: []telemetryv1alpha1.TransformSpec{},
				},
			},
			expectErr: false,
		},
		{
			name: "valid runtime additional metrics",
			pipeline: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						Runtime: &telemetryv1alpha1.MetricPipelineRuntimeInput{
							Enabled:           ptr.To(true),
							AdditionalMetrics: []string{"k8s.container.memory_request_utilization", "k8s.container.status.state"},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "invalid runtime additional metric",
			pipeline: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						Runtime: &telemetryv1alpha1.MetricPipelineRuntimeInput{
							Enabled:           ptr.To(true),
							AdditionalMetrics: []string{"invalid.metric.name"},
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

			_, validateCreateErr := validator.ValidateCreate(t.Context(), tt.pipeline)

			oldPipeline := &telemetryv1alpha1.MetricPipeline{}
			_, validateUpdateErr := validator.ValidateUpdate(t.Context(), oldPipeline, tt.pipeline)

			if tt.expectErr {
				assert.Error(t, validateCreateErr)
				assert.Error(t, validateUpdateErr)
			} else {
				assert.NoError(t, validateCreateErr)
				assert.NoError(t, validateUpdateErr)
			}
		})
	}
}

func TestMetricPipelineValidator_ValidateDelete(t *testing.T) {
	validator := &validator{}

	pipeline := &telemetryv1alpha1.MetricPipeline{}

	warnings, err := validator.ValidateDelete(t.Context(), pipeline)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}
