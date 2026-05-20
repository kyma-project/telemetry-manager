package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/assert"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestMetricPipelineValidator_ValidateCreate_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name      string
		pipeline  telemetryv1beta1.MetricPipeline
		expectErr bool
	}{
		{
			name: "valid ottl filter",
			pipeline: telemetryv1beta1.MetricPipeline{
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
			name: "invalid ottl filter - bad OTTL expression",
			pipeline: telemetryv1beta1.MetricPipeline{
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
			name: "empty ottl filters and transforms - should pass",
			pipeline: telemetryv1beta1.MetricPipeline{
				Spec: telemetryv1beta1.MetricPipelineSpec{
					Filters:    []telemetryv1beta1.FilterSpec{},
					Transforms: []telemetryv1beta1.TransformSpec{},
				},
			},
			expectErr: false,
		},
		{
			name: "valid runtime additional metrics",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				WithRuntimeInputAdditionalMetrics("k8s.container.memory_request_utilization", "k8s.container.status.state").
				Build(),
			expectErr: false,
		},
		{
			name: "invalid runtime additional metric",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				WithRuntimeInputAdditionalMetrics("invalid.metric.name").
				Build(),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &validator{}

			_, validateCreateErr := validator.ValidateCreate(t.Context(), &tt.pipeline)

			oldPipeline := &telemetryv1beta1.MetricPipeline{}
			_, validateUpdateErr := validator.ValidateUpdate(t.Context(), oldPipeline, &tt.pipeline)

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

	pipeline := &telemetryv1beta1.MetricPipeline{}

	warnings, err := validator.ValidateDelete(t.Context(), pipeline)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}
