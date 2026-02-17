package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/namespaces"
)

func TestDefault(t *testing.T) {
	sut := defaulter{
		ExcludeNamespaces: namespaces.System(),
		RuntimeInputResources: runtimeInputResourceDefaults{
			Pod:         true,
			Container:   true,
			Node:        true,
			Volume:      true,
			DaemonSet:   true,
			Deployment:  true,
			StatefulSet: true,
			Job:         true,
		},
		DefaultOTLPOutputProtocol: telemetryv1alpha1.OTLPProtocolGRPC,
	}

	tests := []struct {
		name     string
		input    *telemetryv1alpha1.MetricPipeline
		expected *telemetryv1alpha1.MetricPipeline
	}{
		{
			name: "should set default OTLP protocol and OTLP Input if not set",
			input: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Output: telemetryv1alpha1.MetricPipelineOutput{
						OTLP: &telemetryv1alpha1.OTLPOutput{},
					},
				},
			},
			expected: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
					},
					Output: telemetryv1alpha1.MetricPipelineOutput{
						OTLP: &telemetryv1alpha1.OTLPOutput{
							Protocol: telemetryv1alpha1.OTLPProtocolGRPC,
						},
					},
				},
			},
		},
		{
			name: "should not override existing OTLP protocol",
			input: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
					},
					Output: telemetryv1alpha1.MetricPipelineOutput{
						OTLP: &telemetryv1alpha1.OTLPOutput{
							Protocol: telemetryv1alpha1.OTLPProtocolHTTP,
						},
					},
				},
			},
			expected: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
					},
					Output: telemetryv1alpha1.MetricPipelineOutput{
						OTLP: &telemetryv1alpha1.OTLPOutput{
							Protocol: telemetryv1alpha1.OTLPProtocolHTTP,
						},
					},
				},
			},
		},
		{
			name: "should set default namespaces for Prometheus input",
			input: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
						Prometheus: &telemetryv1alpha1.MetricPipelinePrometheusInput{
							Enabled: new(true),
						},
					},
				},
			},
			expected: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
						Prometheus: &telemetryv1alpha1.MetricPipelinePrometheusInput{
							Enabled: new(true),
							Namespaces: &telemetryv1alpha1.NamespaceSelector{
								Exclude: namespaces.System(),
							},
							DiagnosticMetrics: &telemetryv1alpha1.MetricPipelineIstioInputDiagnosticMetrics{Enabled: new(false)},
						},
					},
				},
			},
		},
		{
			name: "should set default namespaces for Istio input",
			input: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
						Istio: &telemetryv1alpha1.MetricPipelineIstioInput{
							Enabled: new(true),
						},
					},
				},
			},
			expected: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
						Istio: &telemetryv1alpha1.MetricPipelineIstioInput{
							Enabled: new(true),
							Namespaces: &telemetryv1alpha1.NamespaceSelector{
								Exclude: namespaces.System(),
							},
							DiagnosticMetrics: &telemetryv1alpha1.MetricPipelineIstioInputDiagnosticMetrics{Enabled: new(false)},
							EnvoyMetrics:      &telemetryv1alpha1.EnvoyMetrics{Enabled: new(false)},
						},
					},
				},
			},
		},

		{
			name: "should set default for Runtime input",
			input: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
						Runtime: &telemetryv1alpha1.MetricPipelineRuntimeInput{
							Enabled: new(true),
						},
					},
				},
			},
			expected: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
						Runtime: &telemetryv1alpha1.MetricPipelineRuntimeInput{
							Enabled: new(true),
							Namespaces: &telemetryv1alpha1.NamespaceSelector{
								Exclude: namespaces.System(),
							},
							Resources: &telemetryv1alpha1.MetricPipelineRuntimeInputResources{
								Pod: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: new(true),
								},

								Container: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: new(true),
								},

								Node: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: new(true),
								},

								Volume: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: new(true),
								},

								Deployment: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: new(true),
								},

								Job: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: new(true),
								},

								StatefulSet: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: new(true),
								},

								DaemonSet: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: new(true),
								},
							},
						},
					},
				},
			},
		},

		{
			name: "should set default for Runtime input except for Pod",
			input: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
						Runtime: &telemetryv1alpha1.MetricPipelineRuntimeInput{
							Enabled: new(true),
							Resources: &telemetryv1alpha1.MetricPipelineRuntimeInputResources{
								Pod: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: new(false),
								},
							},
						},
					},
				},
			},
			expected: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
						Runtime: &telemetryv1alpha1.MetricPipelineRuntimeInput{
							Enabled: new(true),
							Namespaces: &telemetryv1alpha1.NamespaceSelector{
								Exclude: namespaces.System(),
							},
							Resources: &telemetryv1alpha1.MetricPipelineRuntimeInputResources{
								Pod: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: new(false),
								},

								Container: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: new(true),
								},

								Node: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: new(true),
								},

								Volume: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: new(true),
								},

								Deployment: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: new(true),
								},

								Job: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: new(true),
								},

								StatefulSet: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: new(true),
								},

								DaemonSet: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: new(true),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "should not set default for Prometheus input",
			input: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
						Prometheus: &telemetryv1alpha1.MetricPipelinePrometheusInput{
							Enabled: new(false),
						},
					},
				},
			},
			expected: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
						Prometheus: &telemetryv1alpha1.MetricPipelinePrometheusInput{
							Enabled: new(false),
						},
					},
				},
			},
		},
		{
			name: "should not set defaults for Istio input",
			input: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						Istio: &telemetryv1alpha1.MetricPipelineIstioInput{
							Enabled: new(false),
						},
					},
				},
			},
			expected: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
						Istio: &telemetryv1alpha1.MetricPipelineIstioInput{
							Enabled: new(false),
						},
					},
				},
			},
		},
		{
			name: "should not set defaults for Runtime input",
			input: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
						Runtime: &telemetryv1alpha1.MetricPipelineRuntimeInput{
							Enabled: new(false),
						},
					},
				},
			},
			expected: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
						Runtime: &telemetryv1alpha1.MetricPipelineRuntimeInput{
							Enabled: new(false),
						},
					},
				},
			},
		},

		{
			name: "should set default Istio diagnostic metrics if not set",
			input: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
						Istio: &telemetryv1alpha1.MetricPipelineIstioInput{
							Enabled: new(true),
						},
					},
				},
			},
			expected: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
						Istio: &telemetryv1alpha1.MetricPipelineIstioInput{
							Enabled: new(true),
							Namespaces: &telemetryv1alpha1.NamespaceSelector{
								Exclude: namespaces.System(),
							},
							DiagnosticMetrics: &telemetryv1alpha1.MetricPipelineIstioInputDiagnosticMetrics{Enabled: new(false)},
							EnvoyMetrics:      &telemetryv1alpha1.EnvoyMetrics{Enabled: new(false)},
						},
					},
				},
			},
		},
		{
			name: "should set default Istio Envoy metrics if not set",
			input: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
						Istio: &telemetryv1alpha1.MetricPipelineIstioInput{
							Enabled: new(true),
						},
					},
				},
			},
			expected: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
						Istio: &telemetryv1alpha1.MetricPipelineIstioInput{
							Enabled: new(true),
							Namespaces: &telemetryv1alpha1.NamespaceSelector{
								Exclude: namespaces.System(),
							},
							DiagnosticMetrics: &telemetryv1alpha1.MetricPipelineIstioInputDiagnosticMetrics{Enabled: new(false)},
							EnvoyMetrics:      &telemetryv1alpha1.EnvoyMetrics{Enabled: new(false)},
						},
					},
				},
			},
		},
		{
			name: "should not set default Istio Envoy metrics if set",
			input: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
						Istio: &telemetryv1alpha1.MetricPipelineIstioInput{
							Enabled:      new(true),
							EnvoyMetrics: &telemetryv1alpha1.EnvoyMetrics{Enabled: new(true)},
						},
					},
				},
			},
			expected: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
						Istio: &telemetryv1alpha1.MetricPipelineIstioInput{
							Enabled: new(true),
							Namespaces: &telemetryv1alpha1.NamespaceSelector{
								Exclude: namespaces.System(),
							},
							DiagnosticMetrics: &telemetryv1alpha1.MetricPipelineIstioInputDiagnosticMetrics{Enabled: new(false)},
							EnvoyMetrics:      &telemetryv1alpha1.EnvoyMetrics{Enabled: new(true)},
						},
					},
				},
			},
		},
		{
			name: "should set default Prometheus diagnostic metrics if not set",
			input: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
						Prometheus: &telemetryv1alpha1.MetricPipelinePrometheusInput{
							Enabled: new(true),
						},
					},
				},
			},
			expected: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
						Prometheus: &telemetryv1alpha1.MetricPipelinePrometheusInput{
							Enabled: new(true),
							Namespaces: &telemetryv1alpha1.NamespaceSelector{
								Exclude: namespaces.System(),
							},
							DiagnosticMetrics: &telemetryv1alpha1.MetricPipelineIstioInputDiagnosticMetrics{Enabled: new(false)},
						},
					},
				},
			},
		},
		{
			name: "should not set default Prometheus diagnostic metrics if  set",
			input: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
						Prometheus: &telemetryv1alpha1.MetricPipelinePrometheusInput{
							Enabled:           new(true),
							DiagnosticMetrics: &telemetryv1alpha1.MetricPipelineIstioInputDiagnosticMetrics{Enabled: new(true)},
						},
					},
				},
			},
			expected: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						OTLP: &telemetryv1alpha1.OTLPInput{
							Namespaces: &telemetryv1alpha1.NamespaceSelector{},
						},
						Prometheus: &telemetryv1alpha1.MetricPipelinePrometheusInput{
							Enabled: new(true),
							Namespaces: &telemetryv1alpha1.NamespaceSelector{
								Exclude: namespaces.System(),
							},
							DiagnosticMetrics: &telemetryv1alpha1.MetricPipelineIstioInputDiagnosticMetrics{Enabled: new(true)},
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
