package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/namespaces"
)

var v1alpha1LogPipeline = &LogPipeline{
	ObjectMeta: metav1.ObjectMeta{
		Name: "log-pipeline-test",
	},
	Spec: LogPipelineSpec{
		Input: LogPipelineInput{
			Application: &LogPipelineApplicationInput{
				Enabled: ptr.To(true),
				Namespaces: LogPipelineNamespaceSelector{
					Include: []string{"default", "kube-system"},
					Exclude: []string{"kube-public"},
					System:  false,
				},
				Containers: LogPipelineContainerSelector{
					Include: []string{"nginx", "app"},
					Exclude: []string{"sidecar"},
				},
				FluentBitKeepAnnotations: ptr.To(true),
				FluentBitDropLabels:      ptr.To(true),
				KeepOriginalBody:         ptr.To(true),
			},
			OTLP: &OTLPInput{
				Disabled: true,
				Namespaces: &NamespaceSelector{
					Include: []string{"include", "include2"},
					Exclude: []string{"exclude", "exclude2"},
				},
			},
		},
		FluentBitFiles: []FluentBitFile{
			{Name: "file1", Content: "file1-content"},
		},
		FluentBitFilters: []FluentBitFilter{
			{Custom: "name stdout"},
		},
		Transforms: []TransformSpec{
			{
				Conditions: []string{"resource.attributes[\"k8s.pod.name\"] == nil"},
				Statements: []string{"set(resource.attributes[\"k8s.pod.name\"]", "nginx"},
			},
		},
		Filters: []FilterSpec{
			{
				Conditions: []string{"log.time == nil"},
			},
		},
		Output: LogPipelineOutput{
			FluentBitCustom: "custom-output",
			FluentBitHTTP: &FluentBitHTTPOutput{
				Host: ValueType{
					Value: "http://localhost",
				},
				User: &ValueType{
					Value: "user",
				},
				Password: &ValueType{
					ValueFrom: &ValueFromSource{
						SecretKeyRef: &SecretKeyRef{
							Name:      "secret-name",
							Namespace: "secret-namespace",
							Key:       "secret-key",
						},
					},
				},
				URI:      "/ingest/v1beta1/logs",
				Port:     "8080",
				Compress: "on",
				Format:   "json",
				TLS: FluentBitHTTPOutputTLS{
					Disabled:                  true,
					SkipCertificateValidation: true,
					CA: &ValueType{
						Value: "ca",
					},
					Cert: &ValueType{
						Value: "cert",
					},
					Key: &ValueType{
						Value: "key",
					},
				},
				Dedot: true,
			},
			OTLP: &OTLPOutput{
				Protocol: OTLPProtocolGRPC,
				Endpoint: ValueType{
					Value: "localhost:4317",
				},
				Path: "/v1/logs",
				Authentication: &AuthenticationOptions{
					Basic: &BasicAuthOptions{
						User: ValueType{
							Value: "user",
						},
						Password: ValueType{
							Value: "password",
						},
					},
				},
				Headers: []Header{
					{
						Name: "header1",
						ValueType: ValueType{
							Value: "value1",
						},
						Prefix: "prefix1",
					},
					{
						Name: "header2",
						ValueType: ValueType{
							Value: "value2",
						},
						Prefix: "prefix2",
					},
				},
				TLS: &OTLPTLS{
					Insecure:           true,
					InsecureSkipVerify: true,
					CA: &ValueType{
						Value: "ca",
					},
					Cert: &ValueType{
						Value: "cert",
					},
					Key: &ValueType{
						Value: "key",
					},
				},
			},
		},
	},
	Status: LogPipelineStatus{
		Conditions: []metav1.Condition{
			{
				Type:    "LogAgentHealthy",
				Status:  "True",
				Reason:  "FluentBitReady",
				Message: "FluentBit is and collecting logs",
			},
		},
		UnsupportedMode: ptr.To(true),
	},
}

var v1beta1LogPipeline = &telemetryv1beta1.LogPipeline{
	ObjectMeta: metav1.ObjectMeta{
		Name: "log-pipeline-test",
	},
	Spec: telemetryv1beta1.LogPipelineSpec{
		Input: telemetryv1beta1.LogPipelineInput{
			Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
				Enabled: ptr.To(true),
				Namespaces: &telemetryv1beta1.NamespaceSelector{
					Include: []string{"default", "kube-system"},
					Exclude: []string{"kube-public"},
				},
				Containers: &telemetryv1beta1.LogPipelineContainerSelector{
					Include: []string{"nginx", "app"},
					Exclude: []string{"sidecar"},
				},
				FluentBitKeepAnnotations: ptr.To(true),
				FluentBitDropLabels:      ptr.To(true),
				KeepOriginalBody:         ptr.To(true),
			},
			OTLP: &telemetryv1beta1.OTLPInput{
				Enabled: ptr.To(false),
				Namespaces: &telemetryv1beta1.NamespaceSelector{
					Include: []string{"include", "include2"},
					Exclude: []string{"exclude", "exclude2"},
				},
			},
		},
		FluentBitFiles: []telemetryv1beta1.FluentBitFile{
			{Name: "file1", Content: "file1-content"},
		},
		FluentBitFilters: []telemetryv1beta1.FluentBitFilter{
			{Custom: "name stdout"},
		},
		Transforms: []telemetryv1beta1.TransformSpec{
			{
				Conditions: []string{"resource.attributes[\"k8s.pod.name\"] == nil"},
				Statements: []string{"set(resource.attributes[\"k8s.pod.name\"]", "nginx"},
			},
		},
		Filters: []telemetryv1beta1.FilterSpec{
			{
				Conditions: []string{"log.time == nil"},
			},
		},
		Output: telemetryv1beta1.LogPipelineOutput{
			FluentBitCustom: "custom-output",
			FluentBitHTTP: &telemetryv1beta1.FluentBitHTTPOutput{
				Host: telemetryv1beta1.ValueType{
					Value: "http://localhost",
				},
				User: &telemetryv1beta1.ValueType{
					Value: "user",
				},
				Password: &telemetryv1beta1.ValueType{
					ValueFrom: &telemetryv1beta1.ValueFromSource{
						SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
							Name:      "secret-name",
							Namespace: "secret-namespace",
							Key:       "secret-key",
						},
					},
				},
				URI:      "/ingest/v1beta1/logs",
				Port:     "8080",
				Compress: "on",
				Format:   "json",
				TLS: telemetryv1beta1.OutputTLS{
					Insecure:           true,
					InsecureSkipVerify: true,
					CA: &telemetryv1beta1.ValueType{
						Value: "ca",
					},
					Cert: &telemetryv1beta1.ValueType{
						Value: "cert",
					},
					Key: &telemetryv1beta1.ValueType{
						Value: "key",
					},
				},
				Dedot: true,
			},
			OTLP: &telemetryv1beta1.OTLPOutput{
				Protocol: telemetryv1beta1.OTLPProtocolGRPC,
				Endpoint: telemetryv1beta1.ValueType{Value: "localhost:4317"},
				Path:     "/v1/logs",
				Authentication: &telemetryv1beta1.AuthenticationOptions{Basic: &telemetryv1beta1.BasicAuthOptions{
					User: telemetryv1beta1.ValueType{
						Value: "user",
					},
					Password: telemetryv1beta1.ValueType{
						Value: "password",
					},
				}},
				Headers: []telemetryv1beta1.Header{
					{
						Name: "header1",
						ValueType: telemetryv1beta1.ValueType{
							Value: "value1",
						},
						Prefix: "prefix1",
					},
					{
						Name: "header2",
						ValueType: telemetryv1beta1.ValueType{
							Value: "value2",
						},
						Prefix: "prefix2",
					},
				},
				TLS: &telemetryv1beta1.OutputTLS{
					Insecure:           true,
					InsecureSkipVerify: true,
					CA:                 &telemetryv1beta1.ValueType{Value: "ca"},
					Cert:               &telemetryv1beta1.ValueType{Value: "cert"},
					Key:                &telemetryv1beta1.ValueType{Value: "key"},
				},
			},
		},
	},
	Status: telemetryv1beta1.LogPipelineStatus{
		Conditions: []metav1.Condition{
			{
				Type:    "LogAgentHealthy",
				Status:  "True",
				Reason:  "FluentBitReady",
				Message: "FluentBit is and collecting logs",
			},
		},
		UnsupportedMode: ptr.To(true),
	},
}

func TestLogPipelineConvertTo(t *testing.T) {
	tests := []struct {
		name     string
		input    *LogPipeline
		expected *telemetryv1beta1.LogPipeline
	}{
		{
			name: "should sanitize otlp namespace selectors",
			input: &LogPipeline{
				Spec: LogPipelineSpec{
					Input: LogPipelineInput{
						OTLP: &OTLPInput{
							Namespaces: &NamespaceSelector{
								Include: []string{"valid-ns", "Invalid_NS", "another-valid-ns"},
								Exclude: []string{"valid-excluded", "Invalid@NS", "another-valid-excluded"},
							},
						},
					},
				},
			},
			expected: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						OTLP: &telemetryv1beta1.OTLPInput{
							Enabled: ptr.To(true),
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
			name: "should sanitize application namespace selectors",
			input: &LogPipeline{
				Spec: LogPipelineSpec{
					Input: LogPipelineInput{
						Application: &LogPipelineApplicationInput{
							Namespaces: LogPipelineNamespaceSelector{
								Include: []string{"valid-ns", "Invalid_NS", "another-valid-ns"},
								Exclude: []string{"valid-excluded", "Invalid@NS", "another-valid-excluded"},
							},
						},
					},
				},
			},
			expected: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
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
			name: "should fill default exclude if nothing is defined",
			input: &LogPipeline{
				Spec: LogPipelineSpec{
					Input: LogPipelineInput{
						Application: &LogPipelineApplicationInput{
							Namespaces: LogPipelineNamespaceSelector{},
						},
					},
				},
			},
			expected: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Namespaces: &telemetryv1beta1.NamespaceSelector{
								Exclude: namespaces.System(),
							},
						},
					},
				},
			},
		},
		{
			name: "should not fill defaults if includes are defined",
			input: &LogPipeline{
				Spec: LogPipelineSpec{
					Input: LogPipelineInput{
						Application: &LogPipelineApplicationInput{
							Namespaces: LogPipelineNamespaceSelector{
								Include: []string{"test"},
							},
						},
					},
				},
			},
			expected: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Namespaces: &telemetryv1beta1.NamespaceSelector{
								Include: []string{"test"},
							},
						},
					},
				},
			},
		},
		{
			name: "should not fill defaults if excludes are defined",
			input: &LogPipeline{
				Spec: LogPipelineSpec{
					Input: LogPipelineInput{
						Application: &LogPipelineApplicationInput{
							Namespaces: LogPipelineNamespaceSelector{
								Exclude: []string{"test"},
							},
						},
					},
				},
			},
			expected: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Namespaces: &telemetryv1beta1.NamespaceSelector{
								Exclude: []string{"test"},
							},
						},
					},
				},
			},
		},
		{
			name: "should not fill defaults if system is true",
			input: &LogPipeline{
				Spec: LogPipelineSpec{
					Input: LogPipelineInput{
						Application: &LogPipelineApplicationInput{
							Namespaces: LogPipelineNamespaceSelector{
								System: true,
							},
						},
					},
				},
			},
			expected: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Namespaces: &telemetryv1beta1.NamespaceSelector{},
						},
					},
				},
			},
		},
		{
			name:     "should convert all fields",
			input:    v1alpha1LogPipeline.DeepCopy(),
			expected: v1beta1LogPipeline.DeepCopy(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := &telemetryv1beta1.LogPipeline{}

			err := tt.input.ConvertTo(dst)
			require.NoError(t, err)

			err = marshalData(tt.input, tt.expected)
			require.NoError(t, err)

			require.Equal(t, tt.expected, dst)
		})
	}
}

func TestLogPipelineConvertFrom(t *testing.T) {
	tests := []struct {
		name     string
		input    *telemetryv1beta1.LogPipeline
		expected *LogPipeline
	}{
		{
			name: "should convert namespace selectors without validation",
			input: &telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					Input: telemetryv1beta1.LogPipelineInput{
						Runtime: &telemetryv1beta1.LogPipelineRuntimeInput{
							Namespaces: &telemetryv1beta1.NamespaceSelector{
								Include: []string{"valid-ns", "Invalid_NS"},
								Exclude: []string{"valid-excluded", "Invalid@NS"},
							},
						},
					},
				},
			},
			expected: &LogPipeline{
				Spec: LogPipelineSpec{
					Input: LogPipelineInput{
						Application: &LogPipelineApplicationInput{
							Namespaces: LogPipelineNamespaceSelector{
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
			input:    v1beta1LogPipeline.DeepCopy(),
			expected: v1alpha1LogPipeline.DeepCopy(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := &LogPipeline{}
			err := dst.ConvertFrom(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expected, dst)
		})
	}
}
