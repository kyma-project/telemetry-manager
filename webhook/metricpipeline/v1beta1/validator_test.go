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
			pipeline: testutils.NewMetricPipelineBuilder().
				WithFilter(telemetryv1beta1.FilterSpec{
					Conditions: []string{`IsMatch(metric.name, "envoy") == true`},
				}).
				Build(),
			expectErr: false,
		},
		{
			name: "invalid ottl filter - bad OTTL expression",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithFilter(telemetryv1beta1.FilterSpec{
					Conditions: []string{`IsMatch(metric.name, "envoy") ?= true`},
				}).
				Build(),
			expectErr: true,
		},
		{
			name:      "empty ottl filters and transforms - should pass",
			pipeline:  testutils.NewMetricPipelineBuilder().Build(),
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
			v := newValidator(testutils.SARAllowingClient(t))
			ctx := testutils.ContextWithAdmissionRequest(t, "test-user")

			_, validateCreateErr := v.ValidateCreate(ctx, &tt.pipeline)

			oldPipeline := testutils.NewMetricPipelineBuilder().Build()
			_, validateUpdateErr := v.ValidateUpdate(ctx, &oldPipeline, &tt.pipeline)

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
	v := newValidator(testutils.SARAllowingClient(t))
	pipeline := testutils.NewMetricPipelineBuilder().Build()

	warnings, err := v.ValidateDelete(t.Context(), &pipeline)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestMetricPipelineValidator_SecretAccessDenied(t *testing.T) {
	pipeline := testutils.NewMetricPipelineBuilder().
		WithMetricPipelineOTLPOutput(testutils.OTLPEndpointFromSecret("restricted-secret", "kube-system", "endpoint")).
		Build()

	v := newValidator(testutils.SARDenyingClient(t))
	ctx := testutils.ContextWithAdmissionRequest(t, "test-user")

	_, err := v.ValidateCreate(ctx, &pipeline)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "restricted-secret")

	oldPipeline := testutils.NewMetricPipelineBuilder().Build()
	_, err = v.ValidateUpdate(ctx, &oldPipeline, &pipeline)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "restricted-secret")
}

func TestMetricPipelineValidator_SecretAccessAllowed(t *testing.T) {
	pipeline := testutils.NewMetricPipelineBuilder().
		WithMetricPipelineOTLPOutput(testutils.OTLPEndpointFromSecret("my-secret", "default", "endpoint")).
		Build()

	v := newValidator(testutils.SARAllowingClient(t))
	ctx := testutils.ContextWithAdmissionRequest(t, "test-user")

	_, err := v.ValidateCreate(ctx, &pipeline)
	assert.NoError(t, err)

	oldPipeline := testutils.NewMetricPipelineBuilder().Build()
	_, err = v.ValidateUpdate(ctx, &oldPipeline, &pipeline)
	assert.NoError(t, err)
}
