package v1beta1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

// sarAllowingClient builds a fake client that approves every SubjectAccessReview.
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

func TestTracePipelineValidator_ValidateCreate(t *testing.T) {
	tests := []struct {
		name      string
		pipeline  *telemetryv1beta1.TracePipeline
		expectErr bool
	}{
		{
			name: "valid filter",
			pipeline: &telemetryv1beta1.TracePipeline{
				Spec: telemetryv1beta1.TracePipelineSpec{
					Filters: []telemetryv1beta1.FilterSpec{
						{
							Conditions: []string{`IsMatch(span.attributes, "envoy") == true`},
						},
					},
					Transforms: []telemetryv1beta1.TransformSpec{},
				},
			},
			expectErr: false,
		},
		{
			name: "invalid filter - bad OTTL expression",
			pipeline: &telemetryv1beta1.TracePipeline{
				Spec: telemetryv1beta1.TracePipelineSpec{
					Filters: []telemetryv1beta1.FilterSpec{
						{
							Conditions: []string{`IsMatch(span.attributes, "envoy") ?= true`},
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "empty fields - should pass",
			pipeline: &telemetryv1beta1.TracePipeline{
				Spec: telemetryv1beta1.TracePipelineSpec{
					Filters:    []telemetryv1beta1.FilterSpec{},
					Transforms: []telemetryv1beta1.TransformSpec{},
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := newValidator(sarAllowingClient(t))
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
			name: "valid update",
			oldPipeline: &telemetryv1beta1.TracePipeline{
				Spec: telemetryv1beta1.TracePipelineSpec{
					Filters: []telemetryv1beta1.FilterSpec{},
				},
			},
			newPipeline: &telemetryv1beta1.TracePipeline{
				Spec: telemetryv1beta1.TracePipelineSpec{
					Filters: []telemetryv1beta1.FilterSpec{
						{
							Conditions: []string{`IsMatch(span.attributes, "envoy") == true`},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "invalid update - bad filter",
			oldPipeline: &telemetryv1beta1.TracePipeline{
				Spec: telemetryv1beta1.TracePipelineSpec{
					Filters: []telemetryv1beta1.FilterSpec{},
				},
			},
			newPipeline: &telemetryv1beta1.TracePipeline{
				Spec: telemetryv1beta1.TracePipelineSpec{
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
			v := newValidator(sarAllowingClient(t))
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
	v := newValidator(sarAllowingClient(t))

	pipeline := &telemetryv1beta1.TracePipeline{}

	warnings, err := v.ValidateDelete(t.Context(), pipeline)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}
