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
			pipeline: &telemetryv1beta1.LogPipeline{
				Name: "custom-output",
				Spec: telemetryv1beta1.LogPipelineSpec{
					Output: telemetryv1beta1.LogPipelineOutput{
						FluentBitCustom: "custom-fluentbit-output",
					},
				},
			},
			expectWarnings:    1,
			expectWarningsMsg: renderDeprecationWarning("custom-output", "output.custom"),
		},
		{
			name: "custom filter",
			pipeline: &telemetryv1beta1.LogPipeline{
				Name: "custom-filter",
				Spec: telemetryv1beta1.LogPipelineSpec{
					FluentBitFilters: []telemetryv1beta1.FluentBitFilter{
						{Custom: "custom-filter"},
					},
				},
			},
			expectWarnings:    1,
			expectWarningsMsg: renderDeprecationWarning("custom-filter", "filters"),
		},
		{
			name: "valid filter",
			pipeline: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Filters: []telemetryv1beta1.FilterSpec{
						{
							Conditions: []string{`log.severity_number < SEVERITY_NUMBER_WARN`},
						},
					},
					Transforms: []telemetryv1beta1.TransformSpec{},
				},
			},
			expectErr: false,
		},
		{
			name: "invalid filter - bad OTTL expression",
			pipeline: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Filters: []telemetryv1beta1.FilterSpec{
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
			pipeline: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
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
			oldPipeline: &telemetryv1beta1.LogPipeline{},
			newPipeline: &telemetryv1beta1.LogPipeline{
				Name: "custom-output",
				Spec: telemetryv1beta1.LogPipelineSpec{
					Output: telemetryv1beta1.LogPipelineOutput{
						FluentBitCustom: "custom-fluentbit-output",
					},
				},
			},
			expectWarnings:    1,
			expectWarningsMsg: renderDeprecationWarning("custom-output", "output.custom"),
		},
		{
			name: "valid update",
			oldPipeline: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Filters: []telemetryv1beta1.FilterSpec{},
				},
			},
			newPipeline: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Filters: []telemetryv1beta1.FilterSpec{
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
			oldPipeline: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Filters: []telemetryv1beta1.FilterSpec{},
				},
			},
			newPipeline: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
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
	v := newValidator(sarAllowingClient(t))

	pipeline := &telemetryv1beta1.LogPipeline{}

	warnings, err := v.ValidateDelete(t.Context(), pipeline)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}
