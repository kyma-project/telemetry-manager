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

func sarDenyingClient(t *testing.T) client.Client {
	t.Helper()

	scheme := runtime.NewScheme()
	_ = authorizationv1.AddToScheme(scheme)

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if sar, ok := obj.(*authorizationv1.SubjectAccessReview); ok {
					sar.Status = authorizationv1.SubjectAccessReviewStatus{Allowed: false, Reason: "access denied"}
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
		pipeline  *telemetryv1alpha1.TracePipeline
		expectErr bool
	}{
		{
			name: "valid filter",
			pipeline: &telemetryv1alpha1.TracePipeline{
				Spec: telemetryv1alpha1.TracePipelineSpec{
					Filters: []telemetryv1alpha1.FilterSpec{
						{
							Conditions: []string{`IsMatch(span.attributes, "envoy") == true`},
						},
					},
					Transforms: []telemetryv1alpha1.TransformSpec{},
				},
			},
			expectErr: false,
		},
		{
			name: "invalid filter - bad OTTL expression",
			pipeline: &telemetryv1alpha1.TracePipeline{
				Spec: telemetryv1alpha1.TracePipelineSpec{
					Filters: []telemetryv1alpha1.FilterSpec{
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
			pipeline: &telemetryv1alpha1.TracePipeline{
				Spec: telemetryv1alpha1.TracePipelineSpec{
					Filters:    []telemetryv1alpha1.FilterSpec{},
					Transforms: []telemetryv1alpha1.TransformSpec{},
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
		oldPipeline *telemetryv1alpha1.TracePipeline
		newPipeline *telemetryv1alpha1.TracePipeline
		expectErr   bool
	}{
		{
			name: "valid update",
			oldPipeline: &telemetryv1alpha1.TracePipeline{
				Spec: telemetryv1alpha1.TracePipelineSpec{
					Filters: []telemetryv1alpha1.FilterSpec{},
				},
			},
			newPipeline: &telemetryv1alpha1.TracePipeline{
				Spec: telemetryv1alpha1.TracePipelineSpec{
					Filters: []telemetryv1alpha1.FilterSpec{
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
			oldPipeline: &telemetryv1alpha1.TracePipeline{
				Spec: telemetryv1alpha1.TracePipelineSpec{
					Filters: []telemetryv1alpha1.FilterSpec{},
				},
			},
			newPipeline: &telemetryv1alpha1.TracePipeline{
				Spec: telemetryv1alpha1.TracePipelineSpec{
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

	pipeline := &telemetryv1alpha1.TracePipeline{}

	warnings, err := v.ValidateDelete(t.Context(), pipeline)

	assert.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestTracePipelineValidator_SecretAccessDenied(t *testing.T) {
	pipeline := &telemetryv1alpha1.TracePipeline{
		Spec: telemetryv1alpha1.TracePipelineSpec{
			Output: telemetryv1alpha1.TracePipelineOutput{
				OTLP: &telemetryv1alpha1.OTLPOutput{
					Endpoint: telemetryv1alpha1.ValueType{
						ValueFrom: &telemetryv1alpha1.ValueFromSource{
							SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
								Name:      "my-secret",
								Namespace: "kube-system",
								Key:       "token",
							},
						},
					},
				},
			},
		},
	}

	v := newValidator(sarDenyingClient(t))
	ctx := testutils.ContextWithAdmissionRequest(t, "test-user")

	_, err := v.ValidateCreate(ctx, pipeline)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "my-secret")
}

func TestTracePipelineValidator_SecretAccessAllowed(t *testing.T) {
	pipeline := &telemetryv1alpha1.TracePipeline{
		Spec: telemetryv1alpha1.TracePipelineSpec{
			Output: telemetryv1alpha1.TracePipelineOutput{
				OTLP: &telemetryv1alpha1.OTLPOutput{
					Endpoint: telemetryv1alpha1.ValueType{
						ValueFrom: &telemetryv1alpha1.ValueFromSource{
							SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
								Name:      "my-secret",
								Namespace: "default",
								Key:       "endpoint",
							},
						},
					},
					Authentication: &telemetryv1alpha1.AuthenticationOptions{
						Basic: &telemetryv1alpha1.BasicAuthOptions{
							User: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name:      "auth-secret",
										Namespace: "default",
										Key:       "user",
									},
								},
							},
							Password: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name:      "auth-secret",
										Namespace: "default",
										Key:       "password",
									},
								},
							},
						},
					},
					Headers: []telemetryv1alpha1.Header{
						{
							Name: "X-Token",
							ValueType: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name:      "header-secret",
										Namespace: "default",
										Key:       "token",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	v := newValidator(sarAllowingClient(t))
	ctx := testutils.ContextWithAdmissionRequest(t, "test-user")

	_, err := v.ValidateCreate(ctx, pipeline)
	assert.NoError(t, err)
}

func TestTracePipelineValidator_SecretAccessAllowed_OAuth2(t *testing.T) {
	pipeline := &telemetryv1alpha1.TracePipeline{
		Spec: telemetryv1alpha1.TracePipelineSpec{
			Output: telemetryv1alpha1.TracePipelineOutput{
				OTLP: &telemetryv1alpha1.OTLPOutput{
					Endpoint: telemetryv1alpha1.ValueType{Value: "http://collector:4317"},
					Authentication: &telemetryv1alpha1.AuthenticationOptions{
						OAuth2: &telemetryv1alpha1.OAuth2Options{
							TokenURL: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name:      "oauth-secret",
										Namespace: "default",
										Key:       "tokenURL",
									},
								},
							},
							ClientID: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name:      "oauth-secret",
										Namespace: "default",
										Key:       "clientID",
									},
								},
							},
							ClientSecret: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name:      "oauth-secret",
										Namespace: "default",
										Key:       "clientSecret",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	v := newValidator(sarAllowingClient(t))
	ctx := testutils.ContextWithAdmissionRequest(t, "test-user")

	_, err := v.ValidateCreate(ctx, pipeline)
	assert.NoError(t, err)
}

func TestTracePipelineValidator_NilOTLPOutput(t *testing.T) {
	pipeline := &telemetryv1alpha1.TracePipeline{
		Spec: telemetryv1alpha1.TracePipelineSpec{
			Output: telemetryv1alpha1.TracePipelineOutput{
				OTLP: nil,
			},
		},
	}

	v := newValidator(sarAllowingClient(t))
	ctx := testutils.ContextWithAdmissionRequest(t, "test-user")

	_, err := v.ValidateCreate(ctx, pipeline)
	assert.NoError(t, err)
}


