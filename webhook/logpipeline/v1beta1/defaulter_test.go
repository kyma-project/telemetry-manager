package v1beta1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

func TestDefault(t *testing.T) {
	sut := defaulter{
		RuntimeInputEnabled:          true,
		RuntimeInputKeepOriginalBody: true,
		DefaultOTLPOutputProtocol:    telemetryv1beta1.OTLPProtocolGRPC,
	}

	tests := []struct {
		name     string
		input    *telemetryv1beta1.LogPipeline
		expected *telemetryv1beta1.LogPipeline
	}{
		{
			name: "should set default Runtime Input if not set",
			input: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{},
					},
				},
			},
			expected: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Enabled:          ptr.To(true),
							KeepOriginalBody: ptr.To(true),
						},
					},
				},
			},
		},
		{
			name: "should skip default Runtime Input if set",
			input: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							KeepOriginalBody: ptr.To(false),
						},
					},
				},
			},
			expected: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Enabled:          ptr.To(true),
							KeepOriginalBody: ptr.To(false),
						},
					},
				},
			},
		},
		{
			name: "should skip default Runtime Input if disabled",
			input: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Enabled: ptr.To(false),
						},
					},
				},
			},
			expected: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Enabled: ptr.To(false),
						},
					},
				},
			},
		},
		{
			name: "should set default OTLPOutput if not set",
			input: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Output: telemetryv1beta1.LogPipelineOutput{
						OTLP: &telemetryv1beta1.OTLPOutput{},
					},
				},
			},
			expected: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Output: telemetryv1beta1.LogPipelineOutput{
						OTLP: &telemetryv1beta1.OTLPOutput{
							Protocol: telemetryv1beta1.OTLPProtocolGRPC,
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sut.Default(context.Background(), tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, tt.input)
		})
	}
}
