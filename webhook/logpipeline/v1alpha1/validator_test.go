package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
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
				Name: "custom-output",
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
			name: "custom filter",
			pipeline: &telemetryv1alpha1.LogPipeline{
				Name: "custom-filter",
				Spec: telemetryv1alpha1.LogPipelineSpec{
					FluentBitFilters: []telemetryv1alpha1.FluentBitFilter{
						{Custom: "custom-filter-content"},
					},
				},
			},
			expectWarnings:    1,
			expectWarningsMsg: renderDeprecationWarning("custom-filter", "filters"),
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
			v := newValidator(testutils.SARAllowingClient(t))
			ctx := testutils.ContextWithAdmissionRequest(t, "test-user")

			warnings, err := v.ValidateCreate(ctx, tt.pipeline)

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
				Name: "custom-output",
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
			v := newValidator(testutils.SARAllowingClient(t))
			ctx := testutils.ContextWithAdmissionRequest(t, "test-user")

			warnings, err := v.ValidateUpdate(ctx, tt.oldPipeline, tt.newPipeline)

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
	v := newValidator(testutils.SARAllowingClient(t))

	pipeline := &telemetryv1alpha1.LogPipeline{}

	warnings, err := v.ValidateDelete(t.Context(), pipeline)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestLogPipelineValidator_SecretAccessDenied(t *testing.T) {
	pipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			FluentBitVariables: []telemetryv1alpha1.FluentBitVariable{
				{
					Name: "MY_TOKEN",
					ValueFrom: telemetryv1alpha1.ValueFromSource{
						SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
							Name:      "restricted-secret",
							Namespace: "kube-system",
							Key:       "token",
						},
					},
				},
			},
		},
	}

	v := newValidator(testutils.SARDenyingClient(t))
	ctx := testutils.ContextWithAdmissionRequest(t, "test-user")

	_, err := v.ValidateCreate(ctx, pipeline)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "restricted-secret")
}

func TestLogPipelineValidator_SecretAccessAllowed(t *testing.T) {
	pipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			FluentBitVariables: []telemetryv1alpha1.FluentBitVariable{
				{
					Name: "MY_TOKEN",
					ValueFrom: telemetryv1alpha1.ValueFromSource{
						SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
							Name:      "my-secret",
							Namespace: "default",
							Key:       "token",
						},
					},
				},
			},
		},
	}

	v := newValidator(testutils.SARAllowingClient(t))
	ctx := testutils.ContextWithAdmissionRequest(t, "test-user")

	_, err := v.ValidateCreate(ctx, pipeline)
	assert.NoError(t, err)
}
