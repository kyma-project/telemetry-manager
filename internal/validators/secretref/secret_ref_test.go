package secretref

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		refs        []telemetryv1beta1.SecretKeyRef
		expectError error
	}{
		{
			name: "Success",
			refs: []telemetryv1beta1.SecretKeyRef{
				{Name: "my-secret1", Namespace: "default", Key: "myKey1"},
				{Name: "my-secret2", Namespace: "default", Key: "myKey2"},
			},
			expectError: nil,
		},
		{
			name: "SecretNameNotPresent",
			refs: []telemetryv1beta1.SecretKeyRef{
				{Name: "my-secret1", Namespace: "default", Key: "myKey1"},
				{Name: "notExistent", Namespace: "default", Key: "myKey2"},
			},
			expectError: ErrSecretRefNotFound,
		},
		{
			name: "SecretNamespaceNotPresent",
			refs: []telemetryv1beta1.SecretKeyRef{
				{Name: "my-secret1", Namespace: "default", Key: "myKey1"},
				{Name: "my-secret2", Namespace: "notExistent", Key: "myKey2"},
			},
			expectError: ErrSecretRefNotFound,
		},
		{
			name: "SecretKeyNotPresent",
			refs: []telemetryv1beta1.SecretKeyRef{
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
		refs          telemetryv1beta1.SecretKeyRef
		expectError   error
		expectedValue string
	}{
		{
			name: "Success",
			refs: telemetryv1beta1.SecretKeyRef{
				Name: "my-secret1", Namespace: "default", Key: "myKey1",
			},
			expectError:   nil,
			expectedValue: "myValue",
		},
		{
			name: "SecretNameNotPresent",
			refs: telemetryv1beta1.SecretKeyRef{
				Name: "notExistent", Namespace: "default", Key: "myKey1",
			},
			expectError:   ErrSecretRefNotFound,
			expectedValue: "",
		},
		{
			name: "SecretNamespaceNotPresent",
			refs: telemetryv1beta1.SecretKeyRef{
				Name: "my-secret1", Namespace: "notExistent", Key: "myKey1",
			},
			expectError:   ErrSecretRefNotFound,
			expectedValue: "",
		},
		{
			name: "SecretKeyNotPresent",
			refs: telemetryv1beta1.SecretKeyRef{
				Name: "my-secret1", Namespace: "default", Key: "notExistent",
			},
			expectError:   ErrSecretKeyNotFound,
			expectedValue: "",
		},
		{
			name: "SecretRefMissingKey",
			refs: telemetryv1beta1.SecretKeyRef{
				Name: "my-secret1", Namespace: "default",
			},
			expectError:   ErrSecretRefMissingFields,
			expectedValue: "",
		},
		{
			name: "SecretRefMissingName",
			refs: telemetryv1beta1.SecretKeyRef{
				Namespace: "default", Key: "notExistent",
			},
			expectError:   ErrSecretRefMissingFields,
			expectedValue: "",
		},
		{
			name: "SecretRefMissingNamespace",
			refs: telemetryv1beta1.SecretKeyRef{
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

func TestGetTracePipelineRefs(t *testing.T) {
	tests := []struct {
		name         string
		given        *telemetryv1beta1.OTLPOutput
		pipelineName string
		expected     []telemetryv1beta1.SecretKeyRef
	}{
		{
			name:         "only endpoint",
			pipelineName: "test-pipeline",
			given: &telemetryv1beta1.OTLPOutput{
				Endpoint: telemetryv1beta1.ValueType{
					Value: "",
					ValueFrom: &telemetryv1beta1.ValueFromSource{
						SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
							Name: "secret-1",
							Key:  "endpoint",
						}},
				},
			},

			expected: []telemetryv1beta1.SecretKeyRef{
				{Name: "secret-1", Key: "endpoint"},
			},
		},
		{
			name:         "basic auth and header",
			pipelineName: "test-pipeline",
			given: &telemetryv1beta1.OTLPOutput{
				Authentication: &telemetryv1beta1.AuthenticationOptions{
					Basic: &telemetryv1beta1.BasicAuthOptions{
						User: telemetryv1beta1.ValueType{
							Value: "",
							ValueFrom: &telemetryv1beta1.ValueFromSource{
								SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
									Name:      "secret-1",
									Namespace: "default",
									Key:       "user",
								}},
						},
						Password: telemetryv1beta1.ValueType{
							Value: "",
							ValueFrom: &telemetryv1beta1.ValueFromSource{
								SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
									Name:      "secret-2",
									Namespace: "default",
									Key:       "password",
								}},
						},
					},
				},
				Headers: []telemetryv1beta1.Header{
					{
						Name: "header-1",
						ValueType: telemetryv1beta1.ValueType{
							Value: "",
							ValueFrom: &telemetryv1beta1.ValueFromSource{
								SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
									Name:      "secret-3",
									Namespace: "default",
									Key:       "myheader",
								}},
						},
					},
				},
			},

			expected: []telemetryv1beta1.SecretKeyRef{
				{Name: "secret-1", Namespace: "default", Key: "user"},
				{Name: "secret-2", Namespace: "default", Key: "password"},
				{Name: "secret-3", Namespace: "default", Key: "myheader"},
			},
		},
		{
			name:         "basic auth and header (with missing fields)",
			pipelineName: "test-pipeline",
			given: &telemetryv1beta1.OTLPOutput{
				Authentication: &telemetryv1beta1.AuthenticationOptions{
					Basic: &telemetryv1beta1.BasicAuthOptions{
						User: telemetryv1beta1.ValueType{
							Value: "",
							ValueFrom: &telemetryv1beta1.ValueFromSource{
								SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
									Name: "secret-1",
									Key:  "user",
								}},
						},
						Password: telemetryv1beta1.ValueType{
							Value: "",
							ValueFrom: &telemetryv1beta1.ValueFromSource{
								SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
									Namespace: "default",
									Key:       "password",
								}},
						},
					},
				},
				Headers: []telemetryv1beta1.Header{
					{
						Name: "header-1",
						ValueType: telemetryv1beta1.ValueType{
							Value: "",
							ValueFrom: &telemetryv1beta1.ValueFromSource{
								SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
									Name:      "secret-3",
									Namespace: "default",
								}},
						},
					},
				},
			},

			expected: []telemetryv1beta1.SecretKeyRef{
				{Name: "secret-1", Key: "user"},
				{Namespace: "default", Key: "password"},
				{Name: "secret-3", Namespace: "default"},
			},
		},
		{
			name:         "oauth2",
			pipelineName: "test-pipeline",
			given: &telemetryv1beta1.OTLPOutput{
				Authentication: &telemetryv1beta1.AuthenticationOptions{
					OAuth2: &telemetryv1beta1.OAuth2Options{
						TokenURL: telemetryv1beta1.ValueType{
							Value: "",
							ValueFrom: &telemetryv1beta1.ValueFromSource{
								SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
									Name:      "secret-1",
									Namespace: "default",
									Key:       "token-url",
								}},
						},
						ClientID: telemetryv1beta1.ValueType{
							Value: "",
							ValueFrom: &telemetryv1beta1.ValueFromSource{
								SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
									Name:      "secret-2",
									Namespace: "default",
									Key:       "client-id",
								}},
						},
						ClientSecret: telemetryv1beta1.ValueType{
							Value: "",
							ValueFrom: &telemetryv1beta1.ValueFromSource{
								SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
									Name:      "secret-3",
									Namespace: "default",
									Key:       "client-secret",
								}},
						},
					},
				},
			},

			expected: []telemetryv1beta1.SecretKeyRef{
				{Name: "secret-1", Namespace: "default", Key: "token-url"},
				{Name: "secret-2", Namespace: "default", Key: "client-id"},
				{Name: "secret-3", Namespace: "default", Key: "client-secret"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sut := telemetryv1beta1.TracePipeline{ObjectMeta: metav1.ObjectMeta{Name: test.pipelineName}, Spec: telemetryv1beta1.TracePipelineSpec{Output: telemetryv1beta1.TracePipelineOutput{OTLP: test.given}}}
			actual := GetTracePipelineRefs(&sut)
			require.ElementsMatch(t, test.expected, actual)
		})
	}
}

func TestGetMetricPipelineRefs(t *testing.T) {
	tests := []struct {
		name         string
		given        *telemetryv1beta1.OTLPOutput
		pipelineName string
		expected     []telemetryv1beta1.SecretKeyRef
	}{
		{
			name:         "only endpoint",
			pipelineName: "test-pipeline",
			given: &telemetryv1beta1.OTLPOutput{
				Endpoint: telemetryv1beta1.ValueType{
					Value: "",
					ValueFrom: &telemetryv1beta1.ValueFromSource{
						SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
							Name: "secret-1",
							Key:  "endpoint",
						}},
				},
			},

			expected: []telemetryv1beta1.SecretKeyRef{
				{Name: "secret-1", Key: "endpoint"},
			},
		},
		{
			name:         "basic auth and header",
			pipelineName: "test-pipeline",
			given: &telemetryv1beta1.OTLPOutput{
				Authentication: &telemetryv1beta1.AuthenticationOptions{
					Basic: &telemetryv1beta1.BasicAuthOptions{
						User: telemetryv1beta1.ValueType{
							Value: "",
							ValueFrom: &telemetryv1beta1.ValueFromSource{
								SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
									Name:      "secret-1",
									Namespace: "default",
									Key:       "user",
								}},
						},
						Password: telemetryv1beta1.ValueType{
							Value: "",
							ValueFrom: &telemetryv1beta1.ValueFromSource{
								SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
									Name:      "secret-2",
									Namespace: "default",
									Key:       "password",
								}},
						},
					},
				},
				Headers: []telemetryv1beta1.Header{
					{
						Name: "header-1",
						ValueType: telemetryv1beta1.ValueType{
							Value: "",
							ValueFrom: &telemetryv1beta1.ValueFromSource{
								SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
									Name:      "secret-3",
									Namespace: "default",
									Key:       "myheader",
								}},
						},
					},
				},
			},

			expected: []telemetryv1beta1.SecretKeyRef{
				{Name: "secret-1", Namespace: "default", Key: "user"},
				{Name: "secret-2", Namespace: "default", Key: "password"},
				{Name: "secret-3", Namespace: "default", Key: "myheader"},
			},
		},
		{
			name:         "basic auth and header (with missing fields)",
			pipelineName: "test-pipeline",
			given: &telemetryv1beta1.OTLPOutput{
				Authentication: &telemetryv1beta1.AuthenticationOptions{
					Basic: &telemetryv1beta1.BasicAuthOptions{
						User: telemetryv1beta1.ValueType{
							Value: "",
							ValueFrom: &telemetryv1beta1.ValueFromSource{
								SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
									Namespace: "default",
									Key:       "user",
								}},
						},
						Password: telemetryv1beta1.ValueType{
							Value: "",
							ValueFrom: &telemetryv1beta1.ValueFromSource{
								SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
									Name: "secret-2",
									Key:  "password",
								}},
						},
					},
				},
				Headers: []telemetryv1beta1.Header{
					{
						Name: "header-1",
						ValueType: telemetryv1beta1.ValueType{
							Value: "",
							ValueFrom: &telemetryv1beta1.ValueFromSource{
								SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
									Name:      "secret-3",
									Namespace: "default",
								}},
						},
					},
				},
			},

			expected: []telemetryv1beta1.SecretKeyRef{
				{Namespace: "default", Key: "user"},
				{Name: "secret-2", Key: "password"},
				{Name: "secret-3", Namespace: "default"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sut := telemetryv1beta1.MetricPipeline{ObjectMeta: metav1.ObjectMeta{Name: test.pipelineName}, Spec: telemetryv1beta1.MetricPipelineSpec{Output: telemetryv1beta1.MetricPipelineOutput{OTLP: test.given}}}
			actual := GetMetricPipelineRefs(&sut)
			require.ElementsMatch(t, test.expected, actual)
		})
	}
}

func TesGetLogPipelineRefs(t *testing.T) {
	tests := []struct {
		name     string
		given    telemetryv1beta1.LogPipeline
		expected []telemetryv1beta1.SecretKeyRef
	}{
		{
			name: "only variables",
			given: telemetryv1beta1.LogPipeline{
				Spec: telemetryv1beta1.LogPipelineSpec{
					FluentBitVariables: []telemetryv1beta1.FluentBitVariable{
						{
							Name: "password-1",
							ValueFrom: telemetryv1beta1.ValueFromSource{
								SecretKeyRef: &telemetryv1beta1.SecretKeyRef{Name: "secret-1", Key: "password"},
							},
						},
						{
							Name: "password-2",
							ValueFrom: telemetryv1beta1.ValueFromSource{
								SecretKeyRef: &telemetryv1beta1.SecretKeyRef{Name: "secret-2", Key: "password"},
							},
						},
					},
				},
			},

			expected: []telemetryv1beta1.SecretKeyRef{
				{Name: "secret-1", Key: "password"},
				{Name: "secret-2", Key: "password"},
			},
		},
		{
			name: "http output secret refs",
			given: telemetryv1beta1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cls",
				},
				Spec: telemetryv1beta1.LogPipelineSpec{
					Output: telemetryv1beta1.LogPipelineOutput{
						FluentBitHTTP: &telemetryv1beta1.FluentBitHTTPOutput{
							Host: telemetryv1beta1.ValueType{
								ValueFrom: &telemetryv1beta1.ValueFromSource{
									SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
										Name: "creds", Namespace: "default", Key: "host",
									},
								},
							},
							User: &telemetryv1beta1.ValueType{
								ValueFrom: &telemetryv1beta1.ValueFromSource{
									SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
										Name: "creds", Namespace: "default", Key: "user",
									},
								},
							},
							Password: &telemetryv1beta1.ValueType{
								ValueFrom: &telemetryv1beta1.ValueFromSource{
									SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
										Name: "creds", Namespace: "default", Key: "password",
									},
								},
							},
						},
					},
				},
			},
			expected: []telemetryv1beta1.SecretKeyRef{
				{Name: "creds", Namespace: "default", Key: "host"},
				{Name: "creds", Namespace: "default", Key: "user"},
				{Name: "creds", Namespace: "default", Key: "password"},
			},
		},
		{
			name: "http output secret refs (with missing fields)",
			given: telemetryv1beta1.LogPipeline{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cls",
				},
				Spec: telemetryv1beta1.LogPipelineSpec{
					Output: telemetryv1beta1.LogPipelineOutput{
						FluentBitHTTP: &telemetryv1beta1.FluentBitHTTPOutput{
							Host: telemetryv1beta1.ValueType{
								ValueFrom: &telemetryv1beta1.ValueFromSource{
									SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
										Name: "creds", Namespace: "default",
									},
								},
							},
							User: &telemetryv1beta1.ValueType{
								ValueFrom: &telemetryv1beta1.ValueFromSource{
									SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
										Name: "creds", Key: "user",
									},
								},
							},
							Password: &telemetryv1beta1.ValueType{
								ValueFrom: &telemetryv1beta1.ValueFromSource{
									SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
										Namespace: "default", Key: "password",
									},
								},
							},
						},
					},
				},
			},
			expected: []telemetryv1beta1.SecretKeyRef{
				{Name: "creds", Namespace: "default"},
				{Name: "creds", Key: "user"},
				{Namespace: "default", Key: "password"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := GetLogPipelineRefs(&test.given)
			require.ElementsMatch(t, test.expected, actual)
		})
	}
}

func TestGetValue_SecretNotFound(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	ref := telemetryv1beta1.SecretKeyRef{Name: "my-secret1", Namespace: "default", Key: "myKey1"}

	_, err := GetValue(t.Context(), fakeClient, ref)
	require.Error(t, err)
	require.Contains(t, err.Error(), "one or more referenced Secrets are missing")
}

func TestGetSecretRefsInHTTPOutput_TLSFields(t *testing.T) {
	httpOutput := &telemetryv1beta1.FluentBitHTTPOutput{

		TLS: telemetryv1beta1.OutputTLS{
			CA:   &telemetryv1beta1.ValueType{ValueFrom: &telemetryv1beta1.ValueFromSource{SecretKeyRef: &telemetryv1beta1.SecretKeyRef{Name: "ca", Key: "ca"}}},
			Cert: &telemetryv1beta1.ValueType{ValueFrom: &telemetryv1beta1.ValueFromSource{SecretKeyRef: &telemetryv1beta1.SecretKeyRef{Name: "cert", Key: "cert"}}},
			Key:  &telemetryv1beta1.ValueType{ValueFrom: &telemetryv1beta1.ValueFromSource{SecretKeyRef: &telemetryv1beta1.SecretKeyRef{Name: "key", Key: "key"}}},
		},
	}
	refs := getSecretRefsInHTTPOutput(httpOutput)
	require.ElementsMatch(t, []telemetryv1beta1.SecretKeyRef{
		{Name: "ca", Key: "ca"},
		{Name: "cert", Key: "cert"},
		{Name: "key", Key: "key"},
	}, refs)
}

func TestGetSecretRefsInOTLPOutput_TLSFieldsAndInsecure(t *testing.T) {
	otlp := &telemetryv1beta1.OTLPOutput{
		TLS: &telemetryv1beta1.OutputTLS{
			Insecure: false,
			CA:       &telemetryv1beta1.ValueType{ValueFrom: &telemetryv1beta1.ValueFromSource{SecretKeyRef: &telemetryv1beta1.SecretKeyRef{Name: "ca", Key: "ca"}}},
			Cert:     &telemetryv1beta1.ValueType{ValueFrom: &telemetryv1beta1.ValueFromSource{SecretKeyRef: &telemetryv1beta1.SecretKeyRef{Name: "cert", Key: "cert"}}},
			Key:      &telemetryv1beta1.ValueType{ValueFrom: &telemetryv1beta1.ValueFromSource{SecretKeyRef: &telemetryv1beta1.SecretKeyRef{Name: "key", Key: "key"}}},
		},
	}
	refs := getSecretRefsInOTLPOutput(otlp)
	require.ElementsMatch(t, []telemetryv1beta1.SecretKeyRef{
		{Name: "ca", Key: "ca"},
		{Name: "cert", Key: "cert"},
		{Name: "key", Key: "key"},
	}, refs)

	otlp.TLS.Insecure = true
	refs = getSecretRefsInOTLPOutput(otlp)
	require.Empty(t, refs)
}

func TestAppendIfSecretRef_NonSecretValue(t *testing.T) {
	refs := appendIfSecretRef(nil, &telemetryv1beta1.ValueType{Value: "not-a-secret"})
	require.Empty(t, refs)
}
