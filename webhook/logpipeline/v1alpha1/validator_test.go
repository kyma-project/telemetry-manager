package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func TestLogPipelineValidator_ValidateCreate(t *testing.T) {
	tests := []struct {
		name              string
		pipeline          *telemetryv1alpha1.LogPipeline
		expectErr         bool
		expectWarnings    int
		expectWarningsMsg string
	}{
		{
			name: "custom output",
			pipeline: &telemetryv1alpha1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "custom-output",
				},
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.LogPipelineOutput{
						FluentBitCustom: "custom-fluentbit-output",
					},
				},
			},
			expectWarnings:    1,
			expectWarningsMsg: renderDeprecationWarning("custom-output", "output.custom"),
		},
		{
			name: "valid filter",
			pipeline: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Filters: []telemetryv1alpha1.FilterSpec{
						{
							Conditions: []string{`log.severity_number < SEVERITY_NUMBER_WARN`},
						},
					},
					Transforms: []telemetryv1alpha1.TransformSpec{},
				},
			},
			expectErr: false,
		},
		{
			name: "invalid filter - bad OTTL expression",
			pipeline: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Filters: []telemetryv1alpha1.FilterSpec{
						{
							Conditions: []string{`log.severity_number <? SEVERITY_NUMBER_WARN`},
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "empty fields - should pass",
			pipeline: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Filters:    []telemetryv1alpha1.FilterSpec{},
					Transforms: []telemetryv1alpha1.TransformSpec{},
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &LogPipelineValidator{}

			warnings, err := validator.ValidateCreate(t.Context(), tt.pipeline)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectWarnings > 0 {
				assert.Len(t, warnings, tt.expectWarnings)

				if tt.expectWarningsMsg != "" {
					assert.Contains(t, warnings, tt.expectWarningsMsg, "Warnings %s do not contain expected message: '%s'", warnings, tt.expectWarningsMsg)
				}
			}
		})
	}
}

func TestLogPipelineValidator_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name              string
		oldPipeline       *telemetryv1alpha1.LogPipeline
		newPipeline       *telemetryv1alpha1.LogPipeline
		expectErr         bool
		expectWarnings    int
		expectWarningsMsg string
	}{
		{
			name:        "custom output",
			oldPipeline: &telemetryv1alpha1.LogPipeline{},
			newPipeline: &telemetryv1alpha1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "custom-output",
				},
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.LogPipelineOutput{
						FluentBitCustom: "custom-fluentbit-output",
					},
				},
			},
			expectWarnings:    1,
			expectWarningsMsg: renderDeprecationWarning("custom-output", "output.custom"),
		},
		{
			name: "valid update",
			oldPipeline: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Filters: []telemetryv1alpha1.FilterSpec{},
				},
			},
			newPipeline: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Filters: []telemetryv1alpha1.FilterSpec{
						{
							Conditions: []string{`log.severity_number < SEVERITY_NUMBER_WARN`},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "invalid update - bad filter",
			oldPipeline: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Filters: []telemetryv1alpha1.FilterSpec{},
				},
			},
			newPipeline: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
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
			validator := &LogPipelineValidator{}

			warnings, err := validator.ValidateUpdate(t.Context(), tt.oldPipeline, tt.newPipeline)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectWarnings > 0 {
				assert.Len(t, warnings, tt.expectWarnings)

				if tt.expectWarningsMsg != "" {
					assert.Contains(t, warnings, tt.expectWarningsMsg, "Warnings %s do not contain expected message: '%s'", warnings, tt.expectWarningsMsg)
				}
			}
		})
	}
}

func TestLogPipelineValidator_ValidateDelete(t *testing.T) {
	validator := &LogPipelineValidator{}

	pipeline := &telemetryv1alpha1.LogPipeline{}

	warnings, err := validator.ValidateDelete(t.Context(), pipeline)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestLogPipelineValidator_WrongType(t *testing.T) {
	validator := &LogPipelineValidator{}

	// Pass wrong type
	wrongObject := &telemetryv1alpha1.MetricPipeline{}

	warnings, err := validator.ValidateCreate(t.Context(), wrongObject)

	assert.ErrorContains(t, err, "expected a LogPipeline but got")
	assert.Empty(t, warnings)
}
