package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/assert"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func buildLog(b *testutils.LogPipelineBuilder) *telemetryv1beta1.LogPipeline {
	p := b.Build()
	return &p
}

func TestLogPipelineValidator_ValidateCreate(t *testing.T) {
	tests := []struct {
		name              string
		pipeline          *telemetryv1beta1.LogPipeline
		expectErr         bool
		expectWarnings    int
		expectWarningsMsg string
	}{
		{
			name: "custom output",
			pipeline: buildLog(testutils.NewLogPipelineBuilder().
				WithName("custom-output").
				WithCustomOutput("custom-fluentbit-output")),
			expectWarnings:    1,
			expectWarningsMsg: renderDeprecationWarning("custom-output", "output.custom"),
		},
		{
			name: "custom filter",
			pipeline: buildLog(testutils.NewLogPipelineBuilder().
				WithName("custom-filter").
				WithCustomFilter("custom-filter-content").
				WithOTLPOutput()),
			expectWarnings:    1,
			expectWarningsMsg: renderDeprecationWarning("custom-filter", "filters"),
		},
		{
			name: "valid filter",
			pipeline: buildLog(testutils.NewLogPipelineBuilder().
				WithFilter(telemetryv1beta1.FilterSpec{
					Conditions: []string{`log.severity_number < SEVERITY_NUMBER_WARN`},
				})),
			expectErr: false,
		},
		{
			name: "invalid filter - bad OTTL expression",
			pipeline: buildLog(testutils.NewLogPipelineBuilder().
				WithFilter(telemetryv1beta1.FilterSpec{
					Conditions: []string{`log.severity_number <? SEVERITY_NUMBER_WARN`},
				})),
			expectErr: true,
		},
		{
			name:      "empty fields - should pass",
			pipeline:  buildLog(testutils.NewLogPipelineBuilder()),
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
		oldPipeline       *telemetryv1beta1.LogPipeline
		newPipeline       *telemetryv1beta1.LogPipeline
		expectErr         bool
		expectWarnings    int
		expectWarningsMsg string
	}{
		{
			name:        "custom output",
			oldPipeline: buildLog(testutils.NewLogPipelineBuilder()),
			newPipeline: buildLog(testutils.NewLogPipelineBuilder().
				WithName("custom-output").
				WithCustomOutput("custom-fluentbit-output")),
			expectWarnings:    1,
			expectWarningsMsg: renderDeprecationWarning("custom-output", "output.custom"),
		},
		{
			name:        "valid update",
			oldPipeline: buildLog(testutils.NewLogPipelineBuilder()),
			newPipeline: buildLog(testutils.NewLogPipelineBuilder().
				WithFilter(telemetryv1beta1.FilterSpec{
					Conditions: []string{`log.severity_number < SEVERITY_NUMBER_WARN`},
				})),
			expectErr: false,
		},
		{
			name:        "invalid update - bad filter",
			oldPipeline: buildLog(testutils.NewLogPipelineBuilder()),
			newPipeline: buildLog(testutils.NewLogPipelineBuilder().
				WithFilter(telemetryv1beta1.FilterSpec{
					Conditions: []string{`log.severity_number <? SEVERITY_NUMBER_WARN`},
				})),
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
	pipeline := buildLog(testutils.NewLogPipelineBuilder())

	warnings, err := v.ValidateDelete(t.Context(), pipeline)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestLogPipelineValidator_SecretAccessDenied(t *testing.T) {
	pipeline := buildLog(testutils.NewLogPipelineBuilder().
		WithVariable("MY_TOKEN", "restricted-secret", "kube-system", "token"))

	v := newValidator(testutils.SARDenyingClient(t))
	ctx := testutils.ContextWithAdmissionRequest(t, "test-user")

	_, err := v.ValidateCreate(ctx, pipeline)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "restricted-secret")
}

func TestLogPipelineValidator_SecretAccessAllowed(t *testing.T) {
	pipeline := buildLog(testutils.NewLogPipelineBuilder().
		WithVariable("MY_TOKEN", "my-secret", "default", "token"))

	v := newValidator(testutils.SARAllowingClient(t))
	ctx := testutils.ContextWithAdmissionRequest(t, "test-user")

	_, err := v.ValidateCreate(ctx, pipeline)
	assert.NoError(t, err)
}
