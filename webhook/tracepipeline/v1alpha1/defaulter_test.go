package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func TestDefault(t *testing.T) {
	sut := defaulter{
		DefaultOTLPOutputProtocol: telemetryv1alpha1.OTLPProtocolGRPC,
	}

	tests := []struct {
		name     string
		input    *telemetryv1alpha1.TracePipeline
		expected *telemetryv1alpha1.TracePipeline
	}{
		{
			name: "should set default OTLP protocol if not set",
			input: &telemetryv1alpha1.TracePipeline{
				Spec: telemetryv1alpha1.TracePipelineSpec{
					Output: telemetryv1alpha1.TracePipelineOutput{
						OTLP: &telemetryv1alpha1.OTLPOutput{},
					},
				},
			},
			expected: &telemetryv1alpha1.TracePipeline{
				Spec: telemetryv1alpha1.TracePipelineSpec{
					Output: telemetryv1alpha1.TracePipelineOutput{
						OTLP: &telemetryv1alpha1.OTLPOutput{
							Protocol: telemetryv1alpha1.OTLPProtocolGRPC,
						},
					},
				},
			},
		},
		{
			name: "should not override existing OTLP protocol",
			input: &telemetryv1alpha1.TracePipeline{
				Spec: telemetryv1alpha1.TracePipelineSpec{
					Output: telemetryv1alpha1.TracePipelineOutput{
						OTLP: &telemetryv1alpha1.OTLPOutput{
							Protocol: telemetryv1alpha1.OTLPProtocolHTTP,
						},
					},
				},
			},
			expected: &telemetryv1alpha1.TracePipeline{
				Spec: telemetryv1alpha1.TracePipelineSpec{
					Output: telemetryv1alpha1.TracePipelineOutput{
						OTLP: &telemetryv1alpha1.OTLPOutput{
							Protocol: telemetryv1alpha1.OTLPProtocolHTTP,
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
