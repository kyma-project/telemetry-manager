package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func TestDefault(t *testing.T) {
	sut := defaulter{
		ApplicationInputEnabled:          true,
		ApplicationInputKeepOriginalBody: true,
		DefaultOTLPProtocol:              "grpc",
	}

	tests := []struct {
		name     string
		input    *telemetryv1alpha1.LogPipeline
		expected *telemetryv1alpha1.LogPipeline
	}{
		{
			name: "should set default ApplicationInput if not set",
			input: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Input: telemetryv1alpha1.LogPipelineInput{},
				},
			},
			expected: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Input: telemetryv1alpha1.LogPipelineInput{
						Application: &telemetryv1alpha1.LogPipelineApplicationInput{
							Enabled:          ptr.To(true),
							KeepOriginalBody: ptr.To(true),
						},
					},
				},
			},
		},
		{
			name: "should skip default ApplicationInput if set",
			input: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Input: telemetryv1alpha1.LogPipelineInput{
						Application: &telemetryv1alpha1.LogPipelineApplicationInput{
							KeepOriginalBody: ptr.To(false),
						},
					},
				},
			},
			expected: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Input: telemetryv1alpha1.LogPipelineInput{
						Application: &telemetryv1alpha1.LogPipelineApplicationInput{
							Enabled:          ptr.To(true),
							KeepOriginalBody: ptr.To(false),
						},
					},
				},
			},
		},
		{
			name: "should set keepOriginalBody if only ApplicationInput enabled is set",
			input: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Input: telemetryv1alpha1.LogPipelineInput{
						Application: &telemetryv1alpha1.LogPipelineApplicationInput{
							Enabled: ptr.To(false),
						},
					},
				},
			},
			expected: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Input: telemetryv1alpha1.LogPipelineInput{
						Application: &telemetryv1alpha1.LogPipelineApplicationInput{
							Enabled:          ptr.To(false),
							KeepOriginalBody: ptr.To(true),
						},
					},
				},
			},
		},
		{
			name: "should set default OTLP protocol if not set",
			input: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.LogPipelineOutput{
						OTLP: &telemetryv1alpha1.OTLPOutput{},
					},
				},
			},
			expected: &telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Input: telemetryv1alpha1.LogPipelineInput{
						Application: &telemetryv1alpha1.LogPipelineApplicationInput{
							Enabled:          ptr.To(true),
							KeepOriginalBody: ptr.To(true),
						},
					},
					Output: telemetryv1alpha1.LogPipelineOutput{
						OTLP: &telemetryv1alpha1.OTLPOutput{
							Protocol: "grpc",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sut.Default(t.Context(), tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, tt.input)
		})
	}
}
