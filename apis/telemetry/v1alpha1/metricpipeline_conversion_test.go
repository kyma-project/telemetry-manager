package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

var v1alpha1MetricPipeline = &MetricPipeline{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "full-pipeline",
		Namespace: "default",
	},
	Spec: MetricPipelineSpec{
		Input: MetricPipelineInput{
			Runtime: &MetricPipelineRuntimeInput{
				Enabled: new(true),
				Namespaces: &NamespaceSelector{
					Include: []string{"ns-1", "ns-2"},
					Exclude: []string{"ns-3"},
				},
				Resources: &MetricPipelineRuntimeInputResources{
					Pod: &MetricPipelineRuntimeInputResource{
						Enabled: new(false),
					},
					Container: &MetricPipelineRuntimeInputResource{
						Enabled: new(false),
					},
					Node: &MetricPipelineRuntimeInputResource{
						Enabled: new(false),
					},
					Volume: &MetricPipelineRuntimeInputResource{
						Enabled: new(false),
					},
					Deployment: &MetricPipelineRuntimeInputResource{
						Enabled: new(false),
					},
					DaemonSet: &MetricPipelineRuntimeInputResource{
						Enabled: new(false),
					},
					StatefulSet: &MetricPipelineRuntimeInputResource{
						Enabled: new(false),
					},
					Job: &MetricPipelineRuntimeInputResource{
						Enabled: new(false),
					},
				},
			},
			Istio: &MetricPipelineIstioInput{
				Enabled: new(false),
				Namespaces: &NamespaceSelector{
					Include: []string{"app-ns-1"},
					Exclude: []string{"app-ns-2"},
				},
				DiagnosticMetrics: &MetricPipelineIstioInputDiagnosticMetrics{
					Enabled: new(true),
				},
				EnvoyMetrics: &EnvoyMetrics{
					Enabled: new(true),
				},
			},
			Prometheus: &MetricPipelinePrometheusInput{
				Enabled: new(true),
				Namespaces: &NamespaceSelector{
					Include: []string{"prom-ns-1"},
					Exclude: []string{"prom-ns-2"},
				},
				DiagnosticMetrics: &MetricPipelineIstioInputDiagnosticMetrics{
					Enabled: new(true),
				},
			},
			OTLP: &OTLPInput{
				Disabled: true,
				Namespaces: &NamespaceSelector{
					Include: []string{"otlp-ns-1"},
					Exclude: []string{"otlp-ns-2"},
				},
			},
		},
		Output: MetricPipelineOutput{
			OTLP: &OTLPOutput{
				Endpoint: ValueType{
					Value: "otlp-collector:4317",
				},
				TLS: &OTLPTLS{
					Insecure:           true,
					InsecureSkipVerify: true,
					CA:                 &ValueType{Value: "ca-cert"},
					Cert:               &ValueType{Value: "cert"},
					Key:                &ValueType{Value: "key"},
				},
				Headers: []Header{
					{Name: "header1", ValueType: ValueType{Value: "value1"}, Prefix: "myPrefix"},
				},
			},
		},
	},
	Status: MetricPipelineStatus{
		Conditions: []metav1.Condition{
			{
				Type:    "type",
				Status:  "True",
				Reason:  "Ready",
				Message: "message",
			},
		},
	},
}

var v1beta1MetricPipeline = &telemetryv1beta1.MetricPipeline{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "full-pipeline",
		Namespace: "default",
	},
	Spec: telemetryv1beta1.MetricPipelineSpec{
		Input: telemetryv1beta1.MetricPipelineInput{
			Runtime: &telemetryv1beta1.MetricPipelineRuntimeInput{
				Enabled: new(true),
				Namespaces: &telemetryv1beta1.NamespaceSelector{
					Include: []string{"ns-1", "ns-2"},
					Exclude: []string{"ns-3"},
				},
				Resources: &telemetryv1beta1.MetricPipelineRuntimeInputResources{
					Pod: &telemetryv1beta1.MetricPipelineRuntimeInputResource{
						Enabled: new(false),
					},
					Container: &telemetryv1beta1.MetricPipelineRuntimeInputResource{
						Enabled: new(false),
					},
					Node: &telemetryv1beta1.MetricPipelineRuntimeInputResource{
						Enabled: new(false),
					},
					Volume: &telemetryv1beta1.MetricPipelineRuntimeInputResource{
						Enabled: new(false),
					},
					Deployment: &telemetryv1beta1.MetricPipelineRuntimeInputResource{
						Enabled: new(false),
					},
					DaemonSet: &telemetryv1beta1.MetricPipelineRuntimeInputResource{
						Enabled: new(false),
					},
					StatefulSet: &telemetryv1beta1.MetricPipelineRuntimeInputResource{
						Enabled: new(false),
					},
					Job: &telemetryv1beta1.MetricPipelineRuntimeInputResource{
						Enabled: new(false),
					},
				},
			},
			Istio: &telemetryv1beta1.MetricPipelineIstioInput{
				Enabled: new(false),
				Namespaces: &telemetryv1beta1.NamespaceSelector{
					Include: []string{"app-ns-1"},
					Exclude: []string{"app-ns-2"},
				},
				DiagnosticMetrics: &telemetryv1beta1.MetricPipelineIstioInputDiagnosticMetrics{
					Enabled: new(true),
				},
				EnvoyMetrics: &telemetryv1beta1.EnvoyMetrics{
					Enabled: new(true),
				},
			},
			Prometheus: &telemetryv1beta1.MetricPipelinePrometheusInput{
				Enabled: new(true),
				Namespaces: &telemetryv1beta1.NamespaceSelector{
					Include: []string{"prom-ns-1"},
					Exclude: []string{"prom-ns-2"},
				},
				DiagnosticMetrics: &telemetryv1beta1.MetricPipelineIstioInputDiagnosticMetrics{
					Enabled: new(true),
				},
			},
			OTLP: &telemetryv1beta1.OTLPInput{
				Enabled: new(false),
				Namespaces: &telemetryv1beta1.NamespaceSelector{
					Include: []string{"otlp-ns-1"},
					Exclude: []string{"otlp-ns-2"},
				},
			},
		},
		Output: telemetryv1beta1.MetricPipelineOutput{
			OTLP: &telemetryv1beta1.OTLPOutput{
				Endpoint: telemetryv1beta1.ValueType{
					Value: "otlp-collector:4317",
				},
				TLS: &telemetryv1beta1.OutputTLS{
					Insecure:           true,
					InsecureSkipVerify: true,
					CA:                 &telemetryv1beta1.ValueType{Value: "ca-cert"},
					Cert:               &telemetryv1beta1.ValueType{Value: "cert"},
					Key:                &telemetryv1beta1.ValueType{Value: "key"},
				},
				Headers: []telemetryv1beta1.Header{
					{Name: "header1", ValueType: telemetryv1beta1.ValueType{Value: "value1"}, Prefix: "myPrefix"},
				},
			},
		},
	},
	Status: telemetryv1beta1.MetricPipelineStatus{
		Conditions: []metav1.Condition{
			{
				Type:    "type",
				Status:  "True",
				Reason:  "Ready",
				Message: "message",
			},
		},
	},
}

func TestMetricPipelineConvertTo(t *testing.T) {
	tests := []struct {
		name     string
		input    *MetricPipeline
		expected *telemetryv1beta1.MetricPipeline
	}{
		{
			name: "should sanitize namespace selectors",
			input: &MetricPipeline{
				Spec: MetricPipelineSpec{
					Input: MetricPipelineInput{
						Runtime: &MetricPipelineRuntimeInput{
							Namespaces: &NamespaceSelector{
								Include: []string{"valid-ns", "Invalid_NS", "another-valid-ns"},
								Exclude: []string{"valid-excluded", "Invalid@NS", "another-valid-excluded"},
							},
						},
					},
				},
			},
			expected: &telemetryv1beta1.MetricPipeline{
				Spec: telemetryv1beta1.MetricPipelineSpec{
					Input: telemetryv1beta1.MetricPipelineInput{
						Runtime: &telemetryv1beta1.MetricPipelineRuntimeInput{
							Namespaces: &telemetryv1beta1.NamespaceSelector{
								Include: []string{"valid-ns", "another-valid-ns"},
								Exclude: []string{"valid-excluded", "another-valid-excluded"},
							},
						},
					},
				},
			},
		},
		{
			name:     "should convert all fields",
			input:    v1alpha1MetricPipeline,
			expected: v1beta1MetricPipeline,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := &telemetryv1beta1.MetricPipeline{}
			err := tt.input.ConvertTo(dst)
			require.NoError(t, err)
			require.Equal(t, tt.expected, dst)
		})
	}
}

func TestMetricPipelineConvertFrom(t *testing.T) {
	tests := []struct {
		name     string
		input    *telemetryv1beta1.MetricPipeline
		expected *MetricPipeline
	}{
		{
			name: "should convert namespace selectors without validation",
			input: &telemetryv1beta1.MetricPipeline{
				Spec: telemetryv1beta1.MetricPipelineSpec{
					Input: telemetryv1beta1.MetricPipelineInput{
						Runtime: &telemetryv1beta1.MetricPipelineRuntimeInput{
							Namespaces: &telemetryv1beta1.NamespaceSelector{
								Include: []string{"valid-ns", "Invalid_NS"},
								Exclude: []string{"valid-excluded", "Invalid@NS"},
							},
						},
					},
				},
			},
			expected: &MetricPipeline{
				Spec: MetricPipelineSpec{
					Input: MetricPipelineInput{
						Runtime: &MetricPipelineRuntimeInput{
							Namespaces: &NamespaceSelector{
								Include: []string{"valid-ns", "Invalid_NS"},
								Exclude: []string{"valid-excluded", "Invalid@NS"},
							},
						},
					},
				},
			},
		},
		{
			name:     "should convert all fields",
			input:    v1beta1MetricPipeline,
			expected: v1alpha1MetricPipeline,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := &MetricPipeline{}
			err := dst.ConvertFrom(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expected, dst)
		})
	}
}
