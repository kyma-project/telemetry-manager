package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/telemetry-manager/internal/field"
)

func TestLogPipeline_GetSecretRefs(t *testing.T) {
	tests := []struct {
		name     string
		given    LogPipeline
		expected []field.Descriptor
	}{
		{
			name: "only variables",
			given: LogPipeline{
				Spec: LogPipelineSpec{
					Variables: []VariableRef{
						{
							Name: "password-1",
							ValueFrom: ValueFromSource{
								SecretKeyRef: &SecretKeyRef{Name: "secret-1", Key: "password"},
							},
						},
						{
							Name: "password-2",
							ValueFrom: ValueFromSource{
								SecretKeyRef: &SecretKeyRef{Name: "secret-2", Key: "password"},
							},
						},
					},
				},
			},

			expected: []field.Descriptor{
				{
					SecretKeyRef:    field.SecretKeyRef{Name: "secret-1", Key: "password"},
					TargetSecretKey: "password-1",
				},
				{
					SecretKeyRef:    field.SecretKeyRef{Name: "secret-2", Key: "password"},
					TargetSecretKey: "password-2",
				},
			},
		},
		{
			name: "http output secret refs",
			given: LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cls",
				},
				Spec: LogPipelineSpec{
					Output: Output{
						HTTP: &HTTPOutput{
							Host: ValueType{
								ValueFrom: &ValueFromSource{
									SecretKeyRef: &SecretKeyRef{
										Name: "creds", Namespace: "default", Key: "host",
									},
								},
							},
							User: ValueType{
								ValueFrom: &ValueFromSource{
									SecretKeyRef: &SecretKeyRef{
										Name: "creds", Namespace: "default", Key: "user",
									},
								},
							},
							Password: ValueType{
								ValueFrom: &ValueFromSource{
									SecretKeyRef: &SecretKeyRef{
										Name: "creds", Namespace: "default", Key: "password",
									},
								},
							},
						},
					},
				},
			},
			expected: []field.Descriptor{
				{
					SecretKeyRef:    field.SecretKeyRef{Name: "creds", Namespace: "default", Key: "host"},
					TargetSecretKey: "CLS_DEFAULT_CREDS_HOST",
				},
				{
					SecretKeyRef:    field.SecretKeyRef{Name: "creds", Namespace: "default", Key: "user"},
					TargetSecretKey: "CLS_DEFAULT_CREDS_USER",
				},
				{
					SecretKeyRef:    field.SecretKeyRef{Name: "creds", Namespace: "default", Key: "password"},
					TargetSecretKey: "CLS_DEFAULT_CREDS_PASSWORD",
				},
			},
		},
		{
			name: "loki output secret refs",
			given: LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "loki",
				},
				Spec: LogPipelineSpec{
					Output: Output{
						Loki: &LokiOutput{
							URL: ValueType{
								ValueFrom: &ValueFromSource{
									SecretKeyRef: &SecretKeyRef{
										Name: "creds", Namespace: "default", Key: "url",
									},
								},
							},
						},
					},
				},
			},
			expected: []field.Descriptor{
				{
					SecretKeyRef:    field.SecretKeyRef{Name: "creds", Namespace: "default", Key: "url"},
					TargetSecretKey: "LOKI_DEFAULT_CREDS_URL",
				},
			},
		},
		{
			name: "output secret refs and variables",
			given: LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "loki",
				},
				Spec: LogPipelineSpec{
					Output: Output{
						Loki: &LokiOutput{
							URL: ValueType{
								ValueFrom: &ValueFromSource{
									SecretKeyRef: &SecretKeyRef{
										Name: "creds", Namespace: "default", Key: "url",
									},
								},
							},
						},
					},
					Variables: []VariableRef{
						{
							Name: "password-1",
							ValueFrom: ValueFromSource{
								SecretKeyRef: &SecretKeyRef{Name: "secret-1", Key: "password"},
							},
						},
						{
							Name: "password-2",
							ValueFrom: ValueFromSource{
								SecretKeyRef: &SecretKeyRef{Name: "secret-2", Key: "password"},
							},
						},
					},
				},
			},
			expected: []field.Descriptor{
				{
					SecretKeyRef:    field.SecretKeyRef{Name: "creds", Namespace: "default", Key: "url"},
					TargetSecretKey: "LOKI_DEFAULT_CREDS_URL",
				},
				{
					SecretKeyRef:    field.SecretKeyRef{Name: "secret-1", Key: "password"},
					TargetSecretKey: "password-1",
				},
				{
					SecretKeyRef:    field.SecretKeyRef{Name: "secret-2", Key: "password"},
					TargetSecretKey: "password-2",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := test.given.GetSecretRefs()
			require.ElementsMatch(t, test.expected, actual)
		})
	}
}

func TestTracePipeline_GetSecretRefs(t *testing.T) {
	tests := []struct {
		name         string
		given        OtlpOutput
		pipelineName string
		expected     []field.Descriptor
	}{
		{
			name:         "only endpoint",
			pipelineName: "test-pipeline",
			given: OtlpOutput{
				Endpoint: ValueType{
					Value: "",
					ValueFrom: &ValueFromSource{
						SecretKeyRef: &SecretKeyRef{
							Name: "secret-1",
							Key:  "endpoint",
						}},
				},
			},

			expected: []field.Descriptor{
				{
					TargetSecretKey: "secret-1",
					SecretKeyRef: field.SecretKeyRef{
						Name: "secret-1",
						Key:  "endpoint",
					},
				},
			},
		},
		{
			name:         "basic auth and header",
			pipelineName: "test-pipeline",
			given: OtlpOutput{
				Authentication: &AuthenticationOptions{
					Basic: &BasicAuthOptions{
						User: ValueType{
							Value: "",
							ValueFrom: &ValueFromSource{
								SecretKeyRef: &SecretKeyRef{
									Name:      "secret-1",
									Namespace: "default",
									Key:       "user",
								}},
						},
						Password: ValueType{
							Value: "",
							ValueFrom: &ValueFromSource{
								SecretKeyRef: &SecretKeyRef{
									Name:      "secret-2",
									Namespace: "default",
									Key:       "password",
								}},
						},
					},
				},
				Headers: []Header{
					{
						Name: "header-1",
						ValueType: ValueType{
							Value: "",
							ValueFrom: &ValueFromSource{
								SecretKeyRef: &SecretKeyRef{
									Name:      "secret-3",
									Namespace: "default",
									Key:       "myheader",
								}},
						},
					},
				},
			},

			expected: []field.Descriptor{
				{
					TargetSecretKey: "TEST_PIPELINE_DEFAULT_SECRET_1_USER",
					SecretKeyRef: field.SecretKeyRef{
						Namespace: "default",
						Name:      "secret-1",
						Key:       "user",
					},
				},
				{
					TargetSecretKey: "TEST_PIPELINE_DEFAULT_SECRET_2_PASSWORD",
					SecretKeyRef: field.SecretKeyRef{
						Namespace: "default",
						Name:      "secret-2",
						Key:       "password",
					},
				},
				{
					TargetSecretKey: "TEST_PIPELINE_DEFAULT_SECRET_3_MYHEADER",
					SecretKeyRef: field.SecretKeyRef{
						Namespace: "default",
						Name:      "secret-3",
						Key:       "myheader",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sut := TracePipeline{ObjectMeta: metav1.ObjectMeta{Name: test.pipelineName}, Spec: TracePipelineSpec{Output: TracePipelineOutput{Otlp: &test.given}}}
			actual := sut.GetSecretRefs()
			require.ElementsMatch(t, test.expected, actual)
		})
	}
}

func TestMetricPipeline_GetSecretRefs(t *testing.T) {
	tests := []struct {
		name         string
		given        OtlpOutput
		pipelineName string
		expected     []field.Descriptor
	}{
		{
			name:         "only endpoint",
			pipelineName: "test-pipeline",
			given: OtlpOutput{
				Endpoint: ValueType{
					Value: "",
					ValueFrom: &ValueFromSource{
						SecretKeyRef: &SecretKeyRef{
							Name: "secret-1",
							Key:  "endpoint",
						}},
				},
			},

			expected: []field.Descriptor{
				{
					TargetSecretKey: "secret-1",
					SecretKeyRef: field.SecretKeyRef{
						Name: "secret-1",
						Key:  "endpoint",
					},
				},
			},
		},
		{
			name:         "basic auth and",
			pipelineName: "test-pipeline",
			given: OtlpOutput{
				Authentication: &AuthenticationOptions{
					Basic: &BasicAuthOptions{
						User: ValueType{
							Value: "",
							ValueFrom: &ValueFromSource{
								SecretKeyRef: &SecretKeyRef{
									Name:      "secret-1",
									Namespace: "default",
									Key:       "user",
								}},
						},
						Password: ValueType{
							Value: "",
							ValueFrom: &ValueFromSource{
								SecretKeyRef: &SecretKeyRef{
									Name:      "secret-2",
									Namespace: "default",
									Key:       "password",
								}},
						},
					},
				},
				Headers: []Header{
					{
						Name: "header-1",
						ValueType: ValueType{
							Value: "",
							ValueFrom: &ValueFromSource{
								SecretKeyRef: &SecretKeyRef{
									Name:      "secret-3",
									Namespace: "default",
									Key:       "myheader",
								}},
						},
					},
				},
			},

			expected: []field.Descriptor{
				{
					TargetSecretKey: "TEST_PIPELINE_DEFAULT_SECRET_1_USER",
					SecretKeyRef: field.SecretKeyRef{
						Namespace: "default",
						Name:      "secret-1",
						Key:       "user",
					},
				},
				{
					TargetSecretKey: "TEST_PIPELINE_DEFAULT_SECRET_2_PASSWORD",
					SecretKeyRef: field.SecretKeyRef{
						Namespace: "default",
						Name:      "secret-2",
						Key:       "password",
					},
				},
				{
					TargetSecretKey: "TEST_PIPELINE_DEFAULT_SECRET_3_MYHEADER",
					SecretKeyRef: field.SecretKeyRef{
						Namespace: "default",
						Name:      "secret-3",
						Key:       "myheader",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sut := MetricPipeline{ObjectMeta: metav1.ObjectMeta{Name: test.pipelineName}, Spec: MetricPipelineSpec{Output: MetricPipelineOutput{Otlp: &test.given}}}
			actual := sut.GetSecretRefs()
			require.ElementsMatch(t, test.expected, actual)
		})
	}
}
