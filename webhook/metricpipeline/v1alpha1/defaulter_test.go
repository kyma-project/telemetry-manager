package v1alpha1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func TestDefault(t *testing.T) {
	sut := defaulter{
		ExcludeNamespaces: []string{"kyma-system", "kube-system", "istio-system", "compass-system"},
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
			name: "should set default OTLP protocol if not set",
			input: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Output: telemetryv1alpha1.MetricPipelineOutput{
						OTLP: &telemetryv1alpha1.OTLPOutput{},
					},
				},
			},
			expected: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
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
					Output: telemetryv1alpha1.MetricPipelineOutput{
						OTLP: &telemetryv1alpha1.OTLPOutput{
							Protocol: telemetryv1alpha1.OTLPProtocolHTTP,
						},
					},
				},
			},
			expected: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
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
						Prometheus: &telemetryv1alpha1.MetricPipelinePrometheusInput{
							Enabled: ptr.To(true),
						},
					},
				},
			},
			expected: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						Prometheus: &telemetryv1alpha1.MetricPipelinePrometheusInput{
							Enabled: ptr.To(true),
							Namespaces: &telemetryv1alpha1.NamespaceSelector{
								Exclude: []string{"kyma-system", "kube-system", "istio-system", "compass-system"},
							},
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
						Istio: &telemetryv1alpha1.MetricPipelineIstioInput{
							Enabled: ptr.To(true),
						},
					},
				},
			},
			expected: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						Istio: &telemetryv1alpha1.MetricPipelineIstioInput{
							Enabled: ptr.To(true),
							Namespaces: &telemetryv1alpha1.NamespaceSelector{
								Exclude: []string{"kyma-system", "kube-system", "istio-system", "compass-system"},
							},
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
						Runtime: &telemetryv1alpha1.MetricPipelineRuntimeInput{
							Enabled: ptr.To(true),
						},
					},
				},
			},
			expected: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						Runtime: &telemetryv1alpha1.MetricPipelineRuntimeInput{
							Enabled: ptr.To(true),
							Namespaces: &telemetryv1alpha1.NamespaceSelector{
								Exclude: []string{"kyma-system", "kube-system", "istio-system", "compass-system"},
							},
							Resources: &telemetryv1alpha1.MetricPipelineRuntimeInputResources{
								Pod: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: ptr.To(true),
								},

								Container: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: ptr.To(true),
								},

								Node: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: ptr.To(true),
								},

								Volume: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: ptr.To(true),
								},

								Deployment: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: ptr.To(true),
								},

								Job: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: ptr.To(true),
								},

								StatefulSet: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: ptr.To(true),
								},

								DaemonSet: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: ptr.To(true),
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
						Runtime: &telemetryv1alpha1.MetricPipelineRuntimeInput{
							Enabled: ptr.To(true),
							Resources: &telemetryv1alpha1.MetricPipelineRuntimeInputResources{
								Pod: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: ptr.To(false),
								},
							},
						},
					},
				},
			},
			expected: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						Runtime: &telemetryv1alpha1.MetricPipelineRuntimeInput{
							Enabled: ptr.To(true),
							Namespaces: &telemetryv1alpha1.NamespaceSelector{
								Exclude: []string{"kyma-system", "kube-system", "istio-system", "compass-system"},
							},
							Resources: &telemetryv1alpha1.MetricPipelineRuntimeInputResources{
								Pod: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: ptr.To(false),
								},

								Container: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: ptr.To(true),
								},

								Node: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: ptr.To(true),
								},

								Volume: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: ptr.To(true),
								},

								Deployment: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: ptr.To(true),
								},

								Job: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: ptr.To(true),
								},

								StatefulSet: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: ptr.To(true),
								},

								DaemonSet: &telemetryv1alpha1.MetricPipelineRuntimeInputResource{
									Enabled: ptr.To(true),
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
						Prometheus: &telemetryv1alpha1.MetricPipelinePrometheusInput{
							Enabled: ptr.To(false),
						},
					},
				},
			},
			expected: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						Prometheus: &telemetryv1alpha1.MetricPipelinePrometheusInput{
							Enabled: ptr.To(false),
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
							Enabled: ptr.To(false),
						},
					},
				},
			},
			expected: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						Istio: &telemetryv1alpha1.MetricPipelineIstioInput{
							Enabled: ptr.To(false),
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
						Runtime: &telemetryv1alpha1.MetricPipelineRuntimeInput{
							Enabled: ptr.To(false),
						},
					},
				},
			},
			expected: &telemetryv1alpha1.MetricPipeline{
				Spec: telemetryv1alpha1.MetricPipelineSpec{
					Input: telemetryv1alpha1.MetricPipelineInput{
						Runtime: &telemetryv1alpha1.MetricPipelineRuntimeInput{
							Enabled: ptr.To(false),
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
