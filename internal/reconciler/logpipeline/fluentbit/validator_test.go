package fluentbit

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/resourcelock"
	"github.com/kyma-project/telemetry-manager/internal/validators/endpoint"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

func TestValidateLogPipelineSpec(t *testing.T) {
	tests := []struct {
		name         string
		logPipeline  *telemetryv1beta1.LogPipeline
		expectError  bool
		errorMessage string
	}{
		// Output validation cases
		{
			name: "valid custom output",
			logPipeline: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Output: telemetryv1beta1.LogPipelineOutput{FluentBitCustom: "name http"},
				},
			},
			expectError: false,
		},
		{
			name: "custom output with forbidden parameter",
			logPipeline: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Output: telemetryv1beta1.LogPipelineOutput{
						FluentBitCustom: `
    name    http
    storage.total_limit_size 10G`,
					},
				},
			},
			expectError:  true,
			errorMessage: "custom output plugin 'http' contains forbidden configuration key 'storage.total_limit_size'",
		},
		{
			name: "custom output missing name",
			logPipeline: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Output: telemetryv1beta1.LogPipelineOutput{FluentBitCustom: "Regex .*"},
				},
			},
			expectError:  true,
			errorMessage: "custom output configuration section must have name attribute",
		},
		// Filter validation cases
		{
			name: "valid custom filter",
			logPipeline: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					FluentBitFilters: []telemetryv1beta1.FluentBitFilter{
						{Custom: "Name grep"},
					},
					Output: telemetryv1beta1.LogPipelineOutput{FluentBitCustom: "Name http"},
				},
			},
			expectError: false,
		},
		{
			name: "custom filter without name",
			logPipeline: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					FluentBitFilters: []telemetryv1beta1.FluentBitFilter{
						{Custom: "foo bar"},
					},
					Output: telemetryv1beta1.LogPipelineOutput{FluentBitCustom: "Name http"},
				},
			},
			expectError:  true,
			errorMessage: "custom filter configuration must have name attribute",
		},
		{
			name: "custom filter with forbidden match condition",
			logPipeline: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					FluentBitFilters: []telemetryv1beta1.FluentBitFilter{
						{Custom: "Name grep\nMatch *"},
					},
					Output: telemetryv1beta1.LogPipelineOutput{FluentBitCustom: "Name http"},
				},
			},
			expectError:  true,
			errorMessage: "custom filter plugin 'grep' contains match condition. Match conditions are forbidden",
		},
		{
			name: "denied filter plugin",
			logPipeline: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					FluentBitFilters: []telemetryv1beta1.FluentBitFilter{
						{Custom: "Name kubernetes"},
					},
					Output: telemetryv1beta1.LogPipelineOutput{FluentBitCustom: "Name http"},
				},
			},
			expectError:  true,
			errorMessage: "custom filter plugin 'kubernetes' is not supported. ",
		},
		// FileName validation cases
		{
			name: "valid files",
			logPipeline: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					FluentBitFiles: []telemetryv1beta1.FluentBitFile{
						{Name: "f1.json", Content: ""},
						{Name: "f2.json", Content: ""},
					},
					Output: telemetryv1beta1.LogPipelineOutput{FluentBitCustom: "Name http"},
				},
			},
			expectError: false,
		},
		{
			name: "duplicate file name",
			logPipeline: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					FluentBitFiles: []telemetryv1beta1.FluentBitFile{
						{Name: "f1.json", Content: ""},
						{Name: "f1.json", Content: ""},
					},
					Output: telemetryv1beta1.LogPipelineOutput{FluentBitCustom: "Name http"},
				},
			},
			expectError:  true,
			errorMessage: "duplicate file names detected please review your pipeline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = telemetryv1beta1.AddToScheme(scheme)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects().Build()

			pipelineValidator := &Validator{
				EndpointValidator:  &endpoint.Validator{Client: fakeClient},
				TLSCertValidator:   tlscert.New(fakeClient),
				SecretRefValidator: &secretref.Validator{Client: fakeClient},
				PipelineLock: resourcelock.NewLocker(
					fakeClient,
					types.NamespacedName{
						Name:      "telemetry-logpipeline-lock",
						Namespace: "test",
					},
					3,
				),
			}

			err := pipelineValidator.Validate(t.Context(), tt.logPipeline)

			if tt.expectError {
				require.EqualError(t, err, tt.errorMessage)
			} else {
				require.Nil(t, err)
			}
		})
	}
}
