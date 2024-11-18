package logpipeline

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/featureflags"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestHandle(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	t.Run("should return a warning when a custom plugin is used", func(t *testing.T) {
		logPipeline := testutils.NewLogPipelineBuilder().WithName("custom-output").WithCustomOutput("Name stdout").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects().Build()

		sut := NewValidatingWebhookHandler(fakeClient, scheme)

		response := sut.Handle(context.Background(), admissionRequestFrom(t, logPipeline))

		require.True(t, response.Allowed)
		require.Contains(t, response.Warnings, "Logpipeline 'custom-output' uses unsupported custom filters or outputs. We recommend changing the pipeline to use supported filters or output. See the documentation: https://kyma-project.io/#/telemetry-manager/user/02-logs")
	})

	t.Run("should validate OTLP input based on output", func(t *testing.T) {
		type args struct {
			name    string
			output  *telemetryv1alpha1.LogPipelineOutput
			input   *telemetryv1alpha1.LogPipelineInput
			allowed bool
			message string
		}

		tests := []args{
			{
				name: "otlp-input-and-output",
				output: &telemetryv1alpha1.LogPipelineOutput{
					Custom: "",
					HTTP:   nil,
					OTLP: &telemetryv1alpha1.OTLPOutput{
						Protocol: "grpc",
						Endpoint: telemetryv1alpha1.ValueType{Value: ""},
						TLS: &telemetryv1alpha1.OTLPTLS{
							Insecure: true,
						},
					},
				},
				input: &telemetryv1alpha1.LogPipelineInput{
					OTLP: &telemetryv1alpha1.OTLPInput{},
				},
				allowed: true,
			},
			{
				name: "otlp-input-and-fluentbit-output",
				output: &telemetryv1alpha1.LogPipelineOutput{
					Custom: "",
					HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{
						Host:   telemetryv1alpha1.ValueType{Value: "127.0.0.1"},
						Port:   "8080",
						URI:    "/",
						Format: "json",
						TLS: telemetryv1alpha1.LogPipelineOutputTLS{
							Disabled:                  true,
							SkipCertificateValidation: true,
						},
					},
				},
				input: &telemetryv1alpha1.LogPipelineInput{
					OTLP: &telemetryv1alpha1.OTLPInput{},
				},
				allowed: false,
				message: "invalid log pipeline definition: cannot use OTLP input for pipeline in FluentBit mode",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				featureflags.Enable(featureflags.LogPipelineOTLP)
				defer featureflags.Disable(featureflags.LogPipelineOTLP)

				logPipeline := testutils.NewLogPipelineBuilder().Build()
				logPipeline.Spec.Output = *tt.output
				logPipeline.Spec.Input = *tt.input

				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects().Build()

				sut := NewValidatingWebhookHandler(fakeClient, scheme)

				response := sut.Handle(context.Background(), admissionRequestFrom(t, logPipeline))
				require.Equal(t, tt.allowed, response.Allowed)

				if !tt.allowed {
					require.EqualValues(t, response.Result.Code, http.StatusBadRequest)
					require.Contains(t, response.Result.Message, tt.message)
				}
			})
		}
	})
}

func admissionRequestFrom(t *testing.T, logPipeline telemetryv1alpha1.LogPipeline) admission.Request {
	t.Helper()

	pipelineJSON, err := json.Marshal(logPipeline)
	if err != nil {
		t.Fatalf("failed to marshal log pipeline: %v", err)
	}

	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{Raw: pipelineJSON},
		},
	}
}
