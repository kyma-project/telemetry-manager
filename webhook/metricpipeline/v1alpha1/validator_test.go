package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
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
							Enabled:           new(true),
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
							Enabled:           new(true),
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
			v := newValidator(testutils.SARAllowingClient(t))
			ctx := testutils.ContextWithAdmissionRequest(t, "test-user")

			_, validateCreateErr := v.ValidateCreate(ctx, tt.pipeline)

			oldPipeline := &telemetryv1alpha1.MetricPipeline{}
			_, validateUpdateErr := v.ValidateUpdate(ctx, oldPipeline, tt.pipeline)

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

	pipeline := &telemetryv1alpha1.MetricPipeline{}

	warnings, err := v.ValidateDelete(t.Context(), pipeline)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestMetricPipelineValidator_SecretAccessDenied(t *testing.T) {
	pipeline := &telemetryv1alpha1.MetricPipeline{
		Spec: telemetryv1alpha1.MetricPipelineSpec{
			Output: telemetryv1alpha1.MetricPipelineOutput{
				OTLP: &telemetryv1alpha1.MetricPipelineOTLPOutput{
					Endpoint: telemetryv1alpha1.ValueType{
						ValueFrom: &telemetryv1alpha1.ValueFromSource{
							SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
								Name:      "restricted-secret",
								Namespace: "kube-system",
								Key:       "endpoint",
							},
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

	_, err = v.ValidateUpdate(ctx, &telemetryv1alpha1.MetricPipeline{}, pipeline)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "restricted-secret")
}

func TestMetricPipelineValidator_SecretAccessAllowed(t *testing.T) {
	pipeline := &telemetryv1alpha1.MetricPipeline{
		Spec: telemetryv1alpha1.MetricPipelineSpec{
			Output: telemetryv1alpha1.MetricPipelineOutput{
				OTLP: &telemetryv1alpha1.MetricPipelineOTLPOutput{
					Endpoint: telemetryv1alpha1.ValueType{
						ValueFrom: &telemetryv1alpha1.ValueFromSource{
							SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
								Name:      "my-secret",
								Namespace: "default",
								Key:       "endpoint",
							},
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

	_, err = v.ValidateUpdate(ctx, &telemetryv1alpha1.MetricPipeline{}, pipeline)
	assert.NoError(t, err)
}
