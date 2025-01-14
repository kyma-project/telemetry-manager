package v1beta1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

func TestDefault(t *testing.T) {
	defaulter := TracePipelineDefaulter{
		DefaultOTLPOutputProtocol: telemetryv1beta1.OTLPProtocolGRPC,
	}

	tests := []struct {
		name     string
		input    *telemetryv1beta1.TracePipeline
		expected *telemetryv1beta1.TracePipeline
	}{
		{
			name: "should set default OTLP protocol if not set",
			input: &telemetryv1beta1.TracePipeline{
				Spec: telemetryv1beta1.TracePipelineSpec{
					Output: telemetryv1beta1.TracePipelineOutput{
						OTLP: &telemetryv1beta1.OTLPOutput{},
					},
				},
			},
			expected: &telemetryv1beta1.TracePipeline{
				Spec: telemetryv1beta1.TracePipelineSpec{
					Output: telemetryv1beta1.TracePipelineOutput{
						OTLP: &telemetryv1beta1.OTLPOutput{
							Protocol: telemetryv1beta1.OTLPProtocolGRPC,
						},
					},
				},
			},
		},
		{
			name: "should not override existing OTLP protocol",
			input: &telemetryv1beta1.TracePipeline{
				Spec: telemetryv1beta1.TracePipelineSpec{
					Output: telemetryv1beta1.TracePipelineOutput{
						OTLP: &telemetryv1beta1.OTLPOutput{
							Protocol: telemetryv1beta1.OTLPProtocolHTTP,
						},
					},
				},
			},
			expected: &telemetryv1beta1.TracePipeline{
				Spec: telemetryv1beta1.TracePipelineSpec{
					Output: telemetryv1beta1.TracePipelineOutput{
						OTLP: &telemetryv1beta1.OTLPOutput{
							Protocol: telemetryv1beta1.OTLPProtocolHTTP,
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := defaulter.Default(context.Background(), tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, tt.input)
		})
	}
}
