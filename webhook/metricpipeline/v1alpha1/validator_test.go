package v1alpha1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func sarAllowingClient(t *testing.T) client.Client {
	t.Helper()

	scheme := runtime.NewScheme()
	_ = authorizationv1.AddToScheme(scheme)

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if sar, ok := obj.(*authorizationv1.SubjectAccessReview); ok {
					sar.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: true}
					return nil
				}
				return c.Create(ctx, obj, opts...)
			},
		}).
		Build()
}

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
			v := newValidator(sarAllowingClient(t))
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
	v := newValidator(sarAllowingClient(t))

	pipeline := &telemetryv1alpha1.MetricPipeline{}

	warnings, err := v.ValidateDelete(t.Context(), pipeline)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}
