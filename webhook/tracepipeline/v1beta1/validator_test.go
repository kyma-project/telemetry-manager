package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/assert"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func buildTrace(b *testutils.TracePipelineBuilder) *telemetryv1beta1.TracePipeline {
	p := b.Build()
	return &p
}

func TestTracePipelineValidator_ValidateCreate(t *testing.T) {
	tests := []struct {
		name      string
		pipeline  *telemetryv1beta1.TracePipeline
		expectErr bool
	}{
		{
			name: "valid filter",
			pipeline: buildTrace(testutils.NewTracePipelineBuilder().
				WithFilter(telemetryv1beta1.FilterSpec{
					Conditions: []string{`IsMatch(span.attributes, "envoy") == true`},
				})),
			expectErr: false,
		},
		{
			name: "invalid filter - bad OTTL expression",
			pipeline: buildTrace(testutils.NewTracePipelineBuilder().
				WithFilter(telemetryv1beta1.FilterSpec{
					Conditions: []string{`IsMatch(span.attributes, "envoy") ?= true`},
				})),
			expectErr: true,
		},
		{
			name:      "empty fields - should pass",
			pipeline:  buildTrace(testutils.NewTracePipelineBuilder()),
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := newValidator(testutils.SARAllowingClient(t))
			ctx := testutils.ContextWithAdmissionRequest(t, "test-user")

			_, err := v.ValidateCreate(ctx, tt.pipeline)

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
		oldPipeline *telemetryv1beta1.TracePipeline
		newPipeline *telemetryv1beta1.TracePipeline
		expectErr   bool
	}{
		{
			name:        "valid update",
			oldPipeline: buildTrace(testutils.NewTracePipelineBuilder()),
			newPipeline: buildTrace(testutils.NewTracePipelineBuilder().
				WithFilter(telemetryv1beta1.FilterSpec{
					Conditions: []string{`IsMatch(span.attributes, "envoy") == true`},
				})),
			expectErr: false,
		},
		{
			name:        "invalid update - bad filter",
			oldPipeline: buildTrace(testutils.NewTracePipelineBuilder()),
			newPipeline: buildTrace(testutils.NewTracePipelineBuilder().
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

			_, err := v.ValidateUpdate(ctx, tt.oldPipeline, tt.newPipeline)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTracePipelineValidator_ValidateDelete(t *testing.T) {
	v := newValidator(testutils.SARAllowingClient(t))
	pipeline := buildTrace(testutils.NewTracePipelineBuilder())

	warnings, err := v.ValidateDelete(t.Context(), pipeline)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestTracePipelineValidator_SecretAccessDenied(t *testing.T) {
	pipeline := buildTrace(testutils.NewTracePipelineBuilder().
		WithOTLPOutput(testutils.OTLPEndpointFromSecret("my-secret", "kube-system", "token")))

	v := newValidator(testutils.SARDenyingClient(t))
	ctx := testutils.ContextWithAdmissionRequest(t, "test-user")

	_, err := v.ValidateCreate(ctx, pipeline)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "my-secret")
}

func TestTracePipelineValidator_SecretAccessAllowed(t *testing.T) {
	pipeline := buildTrace(testutils.NewTracePipelineBuilder().
		WithOTLPOutput(testutils.OTLPEndpointFromSecret("my-secret", "default", "token")))

	v := newValidator(testutils.SARAllowingClient(t))
	ctx := testutils.ContextWithAdmissionRequest(t, "test-user")

	_, err := v.ValidateCreate(ctx, pipeline)
	assert.NoError(t, err)
}
