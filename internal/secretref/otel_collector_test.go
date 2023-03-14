package secretref

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func TestFetchDataForOtlpOutputFromCr(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	pipeline := telemetryv1alpha1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipeline",
		},
		Spec: telemetryv1alpha1.TracePipelineSpec{
			Output: telemetryv1alpha1.TracePipelineOutput{
				Otlp: &telemetryv1alpha1.OtlpOutput{
					Endpoint: telemetryv1alpha1.ValueType{
						Value: "endpoint",
					},
					Headers: []telemetryv1alpha1.Header{
						{
							Name: "Authorization",
							ValueType: telemetryv1alpha1.ValueType{
								Value: "Bearer xyz",
							},
						},
						{
							Name: "Test",
							ValueType: telemetryv1alpha1.ValueType{
								Value: "123",
							},
						},
					},
				},
			},
		},
	}

	data, err := FetchDataForOtlpOutput(context.TODO(), client, pipeline.Spec.Output.Otlp)
	require.NoError(t, err)
	require.Contains(t, data, OtlpEndpointVariable)
	require.Contains(t, data, "HEADER_AUTHORIZATION")
	require.Contains(t, data, "HEADER_TEST")
	require.NotContains(t, data, BasicAuthHeaderVariable)
}

func TestFetchDataForOtlpOutputFromSecret(t *testing.T) {
	data := map[string][]byte{
		"user":     []byte("secret-username"),
		"password": []byte("secret-password"),
		"endpoint": []byte("secret-endpoint"),
		"token":    []byte("Bearer 123"),
		"test":     []byte("123"),
	}
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "default",
		},
		Data: data,
	}
	client := fake.NewClientBuilder().WithObjects(&secret).Build()

	pipeline := telemetryv1alpha1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipeline",
		},
		Spec: telemetryv1alpha1.TracePipelineSpec{
			Output: telemetryv1alpha1.TracePipelineOutput{
				Otlp: &telemetryv1alpha1.OtlpOutput{
					Endpoint: telemetryv1alpha1.ValueType{
						ValueFrom: &telemetryv1alpha1.ValueFromSource{
							SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
								Name:      "my-secret",
								Namespace: "default",
								Key:       "endpoint",
							},
						},
					},
					Authentication: &telemetryv1alpha1.AuthenticationOptions{
						Basic: &telemetryv1alpha1.BasicAuthOptions{
							User: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name:      "my-secret",
										Namespace: "default",
										Key:       "user",
									},
								},
							},
							Password: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name:      "my-secret",
										Namespace: "default",
										Key:       "password",
									},
								},
							},
						},
					},
					Headers: []telemetryv1alpha1.Header{
						{
							Name: "Authorization",
							ValueType: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name:      "my-secret",
										Namespace: "default",
										Key:       "token",
									},
								},
							},
						},
						{
							Name: "Test",
							ValueType: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name:      "my-secret",
										Namespace: "default",
										Key:       "test",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	data, err := FetchDataForOtlpOutput(context.TODO(), client, pipeline.Spec.Output.Otlp)
	require.NoError(t, err)
	require.Contains(t, data, OtlpEndpointVariable)
	require.Equal(t, string(data[OtlpEndpointVariable]), "secret-endpoint")
	require.Contains(t, data, BasicAuthHeaderVariable)
	require.Contains(t, data, "HEADER_AUTHORIZATION")
	require.Contains(t, data, "HEADER_TEST")
	require.Equal(t, string(data[BasicAuthHeaderVariable]), getBasicAuthHeader("secret-username", "secret-password"))
}

func TestFetchDataForOtlpOutputFromSecretWithMissingKey(t *testing.T) {
	data := map[string][]byte{
		"user":     []byte("secret-username"),
		"password": []byte("secret-password"),
	}
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "default",
		},
		Data: data,
	}
	client := fake.NewClientBuilder().WithObjects(&secret).Build()

	pipeline := telemetryv1alpha1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipeline",
		},
		Spec: telemetryv1alpha1.TracePipelineSpec{
			Output: telemetryv1alpha1.TracePipelineOutput{
				Otlp: &telemetryv1alpha1.OtlpOutput{
					Endpoint: telemetryv1alpha1.ValueType{
						ValueFrom: &telemetryv1alpha1.ValueFromSource{
							SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
								Name:      "my-secret",
								Namespace: "default",
								Key:       "endpoint",
							},
						},
					},
					Authentication: &telemetryv1alpha1.AuthenticationOptions{
						Basic: &telemetryv1alpha1.BasicAuthOptions{
							User: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name:      "my-secret",
										Namespace: "default",
										Key:       "user",
									},
								},
							},
							Password: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name:      "my-secret",
										Namespace: "default",
										Key:       "password",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := FetchDataForOtlpOutput(context.TODO(), client, pipeline.Spec.Output.Otlp)
	require.Error(t, err)
}

func TestFetchDataForOtlpOutputSecretDataFromNonExistingSecret(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	pipeline := telemetryv1alpha1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipeline",
		},
		Spec: telemetryv1alpha1.TracePipelineSpec{
			Output: telemetryv1alpha1.TracePipelineOutput{
				Otlp: &telemetryv1alpha1.OtlpOutput{
					Endpoint: telemetryv1alpha1.ValueType{
						ValueFrom: &telemetryv1alpha1.ValueFromSource{
							SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
								Name:      "my-secret",
								Namespace: "default",
								Key:       "myKey",
							},
						},
					},
				},
			},
		},
	}

	_, err := FetchDataForOtlpOutput(context.TODO(), client, pipeline.Spec.Output.Otlp)
	require.Error(t, err)
}

func TestGetRefsInOtlpOutput(t *testing.T) {
	tests := []struct {
		name         string
		given        telemetryv1alpha1.OtlpOutput
		pipelineName string
		expected     []FieldDescriptor
	}{
		{
			name:         "only endpoint",
			pipelineName: "test-pipeline",
			given: telemetryv1alpha1.OtlpOutput{
				Endpoint: telemetryv1alpha1.ValueType{
					Value: "",
					ValueFrom: &telemetryv1alpha1.ValueFromSource{
						SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
							Name: "secret-1",
							Key:  "endpoint",
						}},
				},
			},

			expected: []FieldDescriptor{

				{
					TargetSecretKey: "secret-1",
					SecretKeyRef: telemetryv1alpha1.SecretKeyRef{
						Name: "secret-1",
						Key:  "endpoint",
					},
				},
			},
		},
		{
			name:         "basic auth and",
			pipelineName: "test-pipeline",
			given: telemetryv1alpha1.OtlpOutput{
				Authentication: &telemetryv1alpha1.AuthenticationOptions{
					Basic: &telemetryv1alpha1.BasicAuthOptions{
						User: telemetryv1alpha1.ValueType{
							Value: "",
							ValueFrom: &telemetryv1alpha1.ValueFromSource{
								SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
									Name:      "secret-1",
									Namespace: "default",
									Key:       "user",
								}},
						},
						Password: telemetryv1alpha1.ValueType{
							Value: "",
							ValueFrom: &telemetryv1alpha1.ValueFromSource{
								SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
									Name:      "secret-2",
									Namespace: "default",
									Key:       "password",
								}},
						},
					},
				},
				Headers: []telemetryv1alpha1.Header{
					{
						Name: "header-1",
						ValueType: telemetryv1alpha1.ValueType{
							Value: "",
							ValueFrom: &telemetryv1alpha1.ValueFromSource{
								SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
									Name:      "secret-3",
									Namespace: "default",
									Key:       "myheader",
								}},
						},
					},
				},
			},

			expected: []FieldDescriptor{
				{
					TargetSecretKey: "TEST_PIPELINE_DEFAULT_SECRET_1_USER",
					SecretKeyRef: telemetryv1alpha1.SecretKeyRef{
						Namespace: "default",
						Name:      "secret-1",
						Key:       "user",
					},
				},
				{
					TargetSecretKey: "TEST_PIPELINE_DEFAULT_SECRET_2_PASSWORD",
					SecretKeyRef: telemetryv1alpha1.SecretKeyRef{
						Namespace: "default",
						Name:      "secret-2",
						Key:       "password",
					},
				},
				{
					TargetSecretKey: "TEST_PIPELINE_DEFAULT_SECRET_3_MYHEADER",
					SecretKeyRef: telemetryv1alpha1.SecretKeyRef{
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
			actual := getRefsInOtlpOutput(&test.given, test.pipelineName)
			require.ElementsMatch(t, test.expected, actual)
		})
	}
}
