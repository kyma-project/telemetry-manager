package secretref

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		refs        []telemetryv1alpha1.SecretKeyRef
		expectError error
	}{
		{
			name: "Success",
			refs: []telemetryv1alpha1.SecretKeyRef{
				{Name: "my-secret1", Namespace: "default", Key: "myKey1"},
				{Name: "my-secret2", Namespace: "default", Key: "myKey2"},
			},
			expectError: nil,
		},
		{
			name: "SecretNameNotPresent",
			refs: []telemetryv1alpha1.SecretKeyRef{
				{Name: "my-secret1", Namespace: "default", Key: "myKey1"},
				{Name: "notExistent", Namespace: "default", Key: "myKey2"},
			},
			expectError: ErrSecretRefNotFound,
		},
		{
			name: "SecretNamespaceNotPresent",
			refs: []telemetryv1alpha1.SecretKeyRef{
				{Name: "my-secret1", Namespace: "default", Key: "myKey1"},
				{Name: "my-secret2", Namespace: "notExistent", Key: "myKey2"},
			},
			expectError: ErrSecretRefNotFound,
		},
		{
			name: "SecretKeyNotPresent",
			refs: []telemetryv1alpha1.SecretKeyRef{
				{Name: "my-secret1", Namespace: "default", Key: "myKey1"},
				{Name: "my-secret2", Namespace: "default", Key: "notExistent"},
			},
			expectError: ErrSecretKeyNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			existingSecret1 := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret1",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"myKey1": []byte("myValue"),
				},
			}
			existingSecret2 := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret2",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"myKey2": []byte("myValue"),
				},
			}

			client := fake.NewClientBuilder().WithObjects(&existingSecret1).WithObjects(&existingSecret2).Build()

			secretRefValidator := Validator{
				Client: client,
			}
			err := secretRefValidator.validate(t.Context(), test.refs)
			require.ErrorIs(t, err, test.expectError)
		})
	}
}

func TestGetValue(t *testing.T) {
	tests := []struct {
		name          string
		refs          telemetryv1alpha1.SecretKeyRef
		expectError   error
		expectedValue string
	}{
		{
			name: "Success",
			refs: telemetryv1alpha1.SecretKeyRef{
				Name: "my-secret1", Namespace: "default", Key: "myKey1",
			},
			expectError:   nil,
			expectedValue: "myValue",
		},
		{
			name: "SecretNameNotPresent",
			refs: telemetryv1alpha1.SecretKeyRef{
				Name: "notExistent", Namespace: "default", Key: "myKey1",
			},
			expectError:   ErrSecretRefNotFound,
			expectedValue: "",
		},
		{
			name: "SecretNamespaceNotPresent",
			refs: telemetryv1alpha1.SecretKeyRef{
				Name: "my-secret1", Namespace: "notExistent", Key: "myKey1",
			},
			expectError:   ErrSecretRefNotFound,
			expectedValue: "",
		},
		{
			name: "SecretKeyNotPresent",
			refs: telemetryv1alpha1.SecretKeyRef{
				Name: "my-secret1", Namespace: "default", Key: "notExistent",
			},
			expectError:   ErrSecretKeyNotFound,
			expectedValue: "",
		},
		{
			name: "SecretRefMissingKey",
			refs: telemetryv1alpha1.SecretKeyRef{
				Name: "my-secret1", Namespace: "default",
			},
			expectError:   ErrSecretRefMissingFields,
			expectedValue: "",
		},
		{
			name: "SecretRefMissingName",
			refs: telemetryv1alpha1.SecretKeyRef{
				Namespace: "default", Key: "notExistent",
			},
			expectError:   ErrSecretRefMissingFields,
			expectedValue: "",
		},
		{
			name: "SecretRefMissingNamespace",
			refs: telemetryv1alpha1.SecretKeyRef{
				Name: "my-secret1", Key: "notExistent",
			},
			expectError:   ErrSecretRefMissingFields,
			expectedValue: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			existingSecret1 := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-secret1",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"myKey1": []byte("myValue"),
				},
			}

			client := fake.NewClientBuilder().WithObjects(&existingSecret1).Build()

			result, err := GetValue(t.Context(), client, test.refs)

			require.Equal(t, test.expectedValue, string(result))
			require.ErrorIs(t, err, test.expectError)
		})
	}
}

func TestTracePipeline_GetSecretRefs(t *testing.T) {
	tests := []struct {
		name         string
		given        *telemetryv1alpha1.OTLPOutput
		pipelineName string
		expected     []telemetryv1alpha1.SecretKeyRef
	}{
		{
			name:         "only endpoint",
			pipelineName: "test-pipeline",
			given: &telemetryv1alpha1.OTLPOutput{
				Endpoint: telemetryv1alpha1.ValueType{
					Value: "",
					ValueFrom: &telemetryv1alpha1.ValueFromSource{
						SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
							Name: "secret-1",
							Key:  "endpoint",
						}},
				},
			},

			expected: []telemetryv1alpha1.SecretKeyRef{
				{Name: "secret-1", Key: "endpoint"},
			},
		},
		{
			name:         "basic auth and header",
			pipelineName: "test-pipeline",
			given: &telemetryv1alpha1.OTLPOutput{
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

			expected: []telemetryv1alpha1.SecretKeyRef{
				{Name: "secret-1", Namespace: "default", Key: "user"},
				{Name: "secret-2", Namespace: "default", Key: "password"},
				{Name: "secret-3", Namespace: "default", Key: "myheader"},
			},
		},
		{
			name:         "basic auth and header (with missing fields)",
			pipelineName: "test-pipeline",
			given: &telemetryv1alpha1.OTLPOutput{
				Authentication: &telemetryv1alpha1.AuthenticationOptions{
					Basic: &telemetryv1alpha1.BasicAuthOptions{
						User: telemetryv1alpha1.ValueType{
							Value: "",
							ValueFrom: &telemetryv1alpha1.ValueFromSource{
								SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
									Name: "secret-1",
									Key:  "user",
								}},
						},
						Password: telemetryv1alpha1.ValueType{
							Value: "",
							ValueFrom: &telemetryv1alpha1.ValueFromSource{
								SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
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
								}},
						},
					},
				},
			},

			expected: []telemetryv1alpha1.SecretKeyRef{
				{Name: "secret-1", Key: "user"},
				{Namespace: "default", Key: "password"},
				{Name: "secret-3", Namespace: "default"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sut := telemetryv1alpha1.TracePipeline{ObjectMeta: metav1.ObjectMeta{Name: test.pipelineName}, Spec: telemetryv1alpha1.TracePipelineSpec{Output: telemetryv1alpha1.TracePipelineOutput{OTLP: test.given}}}
			actual := getSecretRefsTracePipeline(&sut)
			require.ElementsMatch(t, test.expected, actual)
		})
	}
}

func TestMetricPipeline_GetSecretRefs(t *testing.T) {
	tests := []struct {
		name         string
		given        *telemetryv1alpha1.OTLPOutput
		pipelineName string
		expected     []telemetryv1alpha1.SecretKeyRef
	}{
		{
			name:         "only endpoint",
			pipelineName: "test-pipeline",
			given: &telemetryv1alpha1.OTLPOutput{
				Endpoint: telemetryv1alpha1.ValueType{
					Value: "",
					ValueFrom: &telemetryv1alpha1.ValueFromSource{
						SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
							Name: "secret-1",
							Key:  "endpoint",
						}},
				},
			},

			expected: []telemetryv1alpha1.SecretKeyRef{
				{Name: "secret-1", Key: "endpoint"},
			},
		},
		{
			name:         "basic auth and header",
			pipelineName: "test-pipeline",
			given: &telemetryv1alpha1.OTLPOutput{
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

			expected: []telemetryv1alpha1.SecretKeyRef{
				{Name: "secret-1", Namespace: "default", Key: "user"},
				{Name: "secret-2", Namespace: "default", Key: "password"},
				{Name: "secret-3", Namespace: "default", Key: "myheader"},
			},
		},
		{
			name:         "basic auth and header (with missing fields)",
			pipelineName: "test-pipeline",
			given: &telemetryv1alpha1.OTLPOutput{
				Authentication: &telemetryv1alpha1.AuthenticationOptions{
					Basic: &telemetryv1alpha1.BasicAuthOptions{
						User: telemetryv1alpha1.ValueType{
							Value: "",
							ValueFrom: &telemetryv1alpha1.ValueFromSource{
								SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
									Namespace: "default",
									Key:       "user",
								}},
						},
						Password: telemetryv1alpha1.ValueType{
							Value: "",
							ValueFrom: &telemetryv1alpha1.ValueFromSource{
								SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
									Name: "secret-2",
									Key:  "password",
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
								}},
						},
					},
				},
			},

			expected: []telemetryv1alpha1.SecretKeyRef{
				{Namespace: "default", Key: "user"},
				{Name: "secret-2", Key: "password"},
				{Name: "secret-3", Namespace: "default"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sut := telemetryv1alpha1.MetricPipeline{ObjectMeta: metav1.ObjectMeta{Name: test.pipelineName}, Spec: telemetryv1alpha1.MetricPipelineSpec{Output: telemetryv1alpha1.MetricPipelineOutput{OTLP: test.given}}}
			actual := getSecretRefsMetricPipeline(&sut)
			require.ElementsMatch(t, test.expected, actual)
		})
	}
}

func TestLogPipeline_GetSecretRefs(t *testing.T) {
	tests := []struct {
		name     string
		given    telemetryv1alpha1.LogPipeline
		expected []telemetryv1alpha1.SecretKeyRef
	}{
		{
			name: "only variables",
			given: telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Variables: []telemetryv1alpha1.LogPipelineVariableRef{
						{
							Name: "password-1",
							ValueFrom: telemetryv1alpha1.ValueFromSource{
								SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{Name: "secret-1", Key: "password"},
							},
						},
						{
							Name: "password-2",
							ValueFrom: telemetryv1alpha1.ValueFromSource{
								SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{Name: "secret-2", Key: "password"},
							},
						},
					},
				},
			},

			expected: []telemetryv1alpha1.SecretKeyRef{
				{Name: "secret-1", Key: "password"},
				{Name: "secret-2", Key: "password"},
			},
		},
		{
			name: "http output secret refs",
			given: telemetryv1alpha1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cls",
				},
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.LogPipelineOutput{
						HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{
							Host: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name: "creds", Namespace: "default", Key: "host",
									},
								},
							},
							User: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name: "creds", Namespace: "default", Key: "user",
									},
								},
							},
							Password: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name: "creds", Namespace: "default", Key: "password",
									},
								},
							},
						},
					},
				},
			},
			expected: []telemetryv1alpha1.SecretKeyRef{
				{Name: "creds", Namespace: "default", Key: "host"},
				{Name: "creds", Namespace: "default", Key: "user"},
				{Name: "creds", Namespace: "default", Key: "password"},
			},
		},
		{
			name: "http output secret refs (with missing fields)",
			given: telemetryv1alpha1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cls",
				},
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.LogPipelineOutput{
						HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{
							Host: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name: "creds", Namespace: "default",
									},
								},
							},
							User: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name: "creds", Key: "user",
									},
								},
							},
							Password: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Namespace: "default", Key: "password",
									},
								},
							},
						},
					},
				},
			},
			expected: []telemetryv1alpha1.SecretKeyRef{
				{Name: "creds", Namespace: "default"},
				{Name: "creds", Key: "user"},
				{Namespace: "default", Key: "password"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := getSecretRefsLogPipeline(&test.given)
			require.ElementsMatch(t, test.expected, actual)
		})
	}
}
