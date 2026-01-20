package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/namespaces"
)

func TestDefault(t *testing.T) {
	sut := defaulter{
		ExcludeNamespaces:            namespaces.System(),
		RuntimeInputEnabled:          true,
		RuntimeInputKeepOriginalBody: true,
		DefaultOTLPOutputProtocol:    telemetryv1beta1.OTLPProtocolGRPC,
		OTLPInputEnabled:             true,
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
					Output: telemetryv1beta1.LogPipelineOutput{
						FluentBitHTTP: &telemetryv1beta1.FluentBitHTTPOutput{
							Host: telemetryv1beta1.ValueType{
								Value: "localhost:4317",
							},
						},
					},
				},
			},
			expected: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Enabled:          ptr.To(true),
							KeepOriginalBody: ptr.To(true),
							Namespaces: &telemetryv1beta1.NamespaceSelector{
								Exclude: namespaces.System(),
							},
						},
					},
					Output: telemetryv1beta1.LogPipelineOutput{
						FluentBitHTTP: &telemetryv1beta1.FluentBitHTTPOutput{
							Host: telemetryv1beta1.ValueType{
								Value: "localhost:4317",
							},
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
					Output: telemetryv1beta1.LogPipelineOutput{
						FluentBitHTTP: &telemetryv1beta1.FluentBitHTTPOutput{
							Host: telemetryv1beta1.ValueType{
								Value: "localhost:4317",
							},
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
							Namespaces: &telemetryv1beta1.NamespaceSelector{
								Exclude: namespaces.System(),
							},
						},
					},
					Output: telemetryv1beta1.LogPipelineOutput{
						FluentBitHTTP: &telemetryv1beta1.FluentBitHTTPOutput{
							Host: telemetryv1beta1.ValueType{
								Value: "localhost:4317",
							},
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
					Output: telemetryv1beta1.LogPipelineOutput{
						FluentBitHTTP: &telemetryv1beta1.FluentBitHTTPOutput{
							Host: telemetryv1beta1.ValueType{
								Value: "localhost:4317",
							},
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
					Output: telemetryv1beta1.LogPipelineOutput{
						FluentBitHTTP: &telemetryv1beta1.FluentBitHTTPOutput{
							Host: telemetryv1beta1.ValueType{
								Value: "localhost:4317",
							},
						},
					},
				},
			},
		},
		{
			name: "should set default OTLPOutput protocol and OTLP Input namespaces if not set",
			input: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Output: telemetryv1beta1.LogPipelineOutput{
						OTLP: &telemetryv1beta1.OTLPOutput{},
					},
				},
			},
			expected: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Enabled:          ptr.To(true),
							KeepOriginalBody: ptr.To(true),
							Namespaces: &telemetryv1beta1.NamespaceSelector{
								Exclude: namespaces.System(),
							},
						},
						OTLP: &telemetryv1beta1.OTLPInput{
							Enabled:    ptr.To(true),
							Namespaces: &telemetryv1beta1.NamespaceSelector{},
						},
					},
					Output: telemetryv1beta1.LogPipelineOutput{
						OTLP: &telemetryv1beta1.OTLPOutput{
							Protocol: telemetryv1beta1.OTLPProtocolGRPC,
						},
					},
				},
			},
		},
		{
			name: "should enable otlp input by default for OTLP output pipelines but do not override existing settings",
			input: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Enabled: ptr.To(false),
						},
						OTLP: &telemetryv1beta1.OTLPInput{
							Namespaces: &telemetryv1beta1.NamespaceSelector{
								Exclude: []string{"custom-namespace"},
							},
						},
					},
					Output: telemetryv1beta1.LogPipelineOutput{
						OTLP: &telemetryv1beta1.OTLPOutput{
							Endpoint: telemetryv1beta1.ValueType{
								Value: "localhost:4317",
							},
							Protocol: telemetryv1beta1.OTLPProtocolGRPC,
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
						OTLP: &telemetryv1beta1.OTLPInput{
							Enabled: ptr.To(true),
							Namespaces: &telemetryv1beta1.NamespaceSelector{
								Exclude: []string{"custom-namespace"},
							},
						},
					},
					Output: telemetryv1beta1.LogPipelineOutput{
						OTLP: &telemetryv1beta1.OTLPOutput{
							Endpoint: telemetryv1beta1.ValueType{
								Value: "localhost:4317",
							},
							Protocol: telemetryv1beta1.OTLPProtocolGRPC,
						},
					},
				},
			},
		},
		{
			name: "should not activate otlp input by default for non-OTLP output pipelines, but runtime input",
			input: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Output: telemetryv1beta1.LogPipelineOutput{
						FluentBitHTTP: &telemetryv1beta1.FluentBitHTTPOutput{
							Host: telemetryv1beta1.ValueType{
								Value: "localhost",
							},
						},
					},
				},
			},
			expected: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Enabled:          ptr.To(true),
							KeepOriginalBody: ptr.To(true),
							Namespaces: &telemetryv1beta1.NamespaceSelector{
								Exclude: namespaces.System(),
							},
						},
					},
					Output: telemetryv1beta1.LogPipelineOutput{
						FluentBitHTTP: &telemetryv1beta1.FluentBitHTTPOutput{
							Host: telemetryv1beta1.ValueType{
								Value: "localhost",
							},
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
