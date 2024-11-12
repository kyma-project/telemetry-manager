package logpipeline

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func TestValidateLogPipelineSpec(t *testing.T) {
	tests := []struct {
		name         string
		logPipeline  *telemetryv1alpha1.LogPipeline
		expectError  bool
		errorMessage string
	}{
		// Output validation cases
		{
			name:         "no output defined",
			logPipeline:  &telemetryv1alpha1.LogPipeline{},
			expectError:  true,
			errorMessage: "no output plugin is defined, you must define one output plugin",
		},
		{
			name: "multiple outputs defined",
			logPipeline: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.LogPipelineOutput{
						Custom: `Name http`,
						HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{
							Host: telemetryv1alpha1.ValueType{Value: "localhost"},
						},
					},
				},
			},
			expectError:  true,
			errorMessage: "multiple output plugins are defined, you must define only one output plugin",
		},
		{
			name: "valid custom output",
			logPipeline: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.LogPipelineOutput{Custom: "name http"},
				},
			},
			expectError: false,
		},
		{
			name: "custom output with forbidden parameter",
			logPipeline: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.LogPipelineOutput{
						Custom: `
    name    http
    storage.total_limit_size 10G`,
					},
				},
			},
			expectError:  true,
			errorMessage: "output plugin 'http' contains forbidden configuration key 'storage.total_limit_size'",
		},
		{
			name: "custom output missing name",
			logPipeline: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.LogPipelineOutput{Custom: "Regex .*"},
				},
			},
			expectError:  true,
			errorMessage: "configuration section must have name attribute",
		},
		{
			name: "both value and valueFrom in HTTP output host",
			logPipeline: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.LogPipelineOutput{
						HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{
							Host: telemetryv1alpha1.ValueType{
								Value: "localhost",
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name:      "foo",
										Namespace: "foo-ns",
										Key:       "foo-key",
									},
								},
							},
						},
					},
				},
			},
			expectError:  true,
			errorMessage: "http output host must have either a value or secret key reference",
		},
		{
			name: "valid HTTP output with ValueFrom",
			logPipeline: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.LogPipelineOutput{
						HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{
							Host: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name:      "foo",
										Namespace: "foo-ns",
										Key:       "foo-key",
									},
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
		// Filter validation cases
		{
			name: "valid custom filter",
			logPipeline: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Filters: []telemetryv1alpha1.LogPipelineFilter{
						{Custom: "Name grep"},
					},
					Output: telemetryv1alpha1.LogPipelineOutput{Custom: "Name http"},
				},
			},
			expectError: false,
		},
		{
			name: "custom filter without name",
			logPipeline: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Filters: []telemetryv1alpha1.LogPipelineFilter{
						{Custom: "foo bar"},
					},
					Output: telemetryv1alpha1.LogPipelineOutput{Custom: "Name http"},
				},
			},
			expectError:  true,
			errorMessage: "configuration section must have name attribute",
		},
		{
			name: "custom filter with forbidden match condition",
			logPipeline: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Filters: []telemetryv1alpha1.LogPipelineFilter{
						{Custom: "Name grep\nMatch *"},
					},
					Output: telemetryv1alpha1.LogPipelineOutput{Custom: "Name http"},
				},
			},
			expectError:  true,
			errorMessage: "filter plugin 'grep' contains match condition. Match conditions are forbidden",
		},
		{
			name: "denied filter plugin",
			logPipeline: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Filters: []telemetryv1alpha1.LogPipelineFilter{
						{Custom: "Name kubernetes"},
					},
					Output: telemetryv1alpha1.LogPipelineOutput{Custom: "Name http"},
				},
			},
			expectError:  true,
			errorMessage: "filter plugin 'kubernetes' is forbidden. ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = telemetryv1alpha1.AddToScheme(scheme)

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects().Build()

			sut := NewValidatingWebhookHandler(fakeClient, scheme)

			response := sut.Handle(context.Background(), admissionRequestFrom(t, *tt.logPipeline))

			if tt.expectError {
				require.False(t, response.Allowed)
				require.EqualValues(t, response.Result.Code, http.StatusBadRequest)
				require.Equal(t, response.Result.Message, tt.errorMessage)
			} else {
				require.True(t, response.Allowed)
			}
		})
	}
}
