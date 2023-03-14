package secretref

import (
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func TestGetRefsInLogPipeline(t *testing.T) {
	tests := []struct {
		name     string
		given    telemetryv1alpha1.LogPipeline
		expected []FieldDescriptor
	}{
		{
			name: "only variables",
			given: telemetryv1alpha1.LogPipeline{
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Variables: []telemetryv1alpha1.VariableRef{
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

			expected: []FieldDescriptor{
				{
					SecretKeyRef:    telemetryv1alpha1.SecretKeyRef{Name: "secret-1", Key: "password"},
					TargetSecretKey: "password-1",
				},
				{
					SecretKeyRef:    telemetryv1alpha1.SecretKeyRef{Name: "secret-2", Key: "password"},
					TargetSecretKey: "password-2",
				},
			},
		},
		{
			name: "http output secret refs",
			given: telemetryv1alpha1.LogPipeline{
				ObjectMeta: v1.ObjectMeta{
					Name: "cls",
				},
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.Output{
						HTTP: &telemetryv1alpha1.HTTPOutput{
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
			expected: []FieldDescriptor{
				{
					SecretKeyRef:    telemetryv1alpha1.SecretKeyRef{Name: "creds", Namespace: "default", Key: "host"},
					TargetSecretKey: "CLS_DEFAULT_CREDS_HOST",
				},
				{
					SecretKeyRef:    telemetryv1alpha1.SecretKeyRef{Name: "creds", Namespace: "default", Key: "user"},
					TargetSecretKey: "CLS_DEFAULT_CREDS_USER",
				},
				{
					SecretKeyRef:    telemetryv1alpha1.SecretKeyRef{Name: "creds", Namespace: "default", Key: "password"},
					TargetSecretKey: "CLS_DEFAULT_CREDS_PASSWORD",
				},
			},
		},
		{
			name: "loki output secret refs",
			given: telemetryv1alpha1.LogPipeline{
				ObjectMeta: v1.ObjectMeta{
					Name: "loki",
				},
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.Output{
						Loki: &telemetryv1alpha1.LokiOutput{
							URL: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name: "creds", Namespace: "default", Key: "url",
									},
								},
							},
						},
					},
				},
			},
			expected: []FieldDescriptor{
				{
					SecretKeyRef:    telemetryv1alpha1.SecretKeyRef{Name: "creds", Namespace: "default", Key: "url"},
					TargetSecretKey: "LOKI_DEFAULT_CREDS_URL",
				},
			},
		},
		{
			name: "output secret refs and variables",
			given: telemetryv1alpha1.LogPipeline{
				ObjectMeta: v1.ObjectMeta{
					Name: "loki",
				},
				Spec: telemetryv1alpha1.LogPipelineSpec{
					Output: telemetryv1alpha1.Output{
						Loki: &telemetryv1alpha1.LokiOutput{
							URL: telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{
									SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
										Name: "creds", Namespace: "default", Key: "url",
									},
								},
							},
						},
					},
					Variables: []telemetryv1alpha1.VariableRef{
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
			expected: []FieldDescriptor{
				{
					SecretKeyRef:    telemetryv1alpha1.SecretKeyRef{Name: "creds", Namespace: "default", Key: "url"},
					TargetSecretKey: "LOKI_DEFAULT_CREDS_URL",
				},
				{
					SecretKeyRef:    telemetryv1alpha1.SecretKeyRef{Name: "secret-1", Key: "password"},
					TargetSecretKey: "password-1",
				},
				{
					SecretKeyRef:    telemetryv1alpha1.SecretKeyRef{Name: "secret-2", Key: "password"},
					TargetSecretKey: "password-2",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := GetRefsInLogPipeline(&test.given)
			require.ElementsMatch(t, test.expected, actual)
		})
	}
}

func TestLogPipelineReferencesSecret(t *testing.T) {
	pipeline := telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.Output{
				Loki: &telemetryv1alpha1.LokiOutput{
					URL: telemetryv1alpha1.ValueType{
						ValueFrom: &telemetryv1alpha1.ValueFromSource{
							SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
								Name: "creds", Namespace: "default", Key: "url",
							},
						},
					},
				},
			},
			Variables: []telemetryv1alpha1.VariableRef{
				{
					Name: "password-1",
					ValueFrom: telemetryv1alpha1.ValueFromSource{
						SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{Name: "secret-1", Namespace: "default", Key: "password"},
					},
				},
				{
					Name: "password-2",
					ValueFrom: telemetryv1alpha1.ValueFromSource{
						SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{Name: "secret-2", Namespace: "default", Key: "password"},
					},
				},
			},
		},
	}

	require.True(t, LogPipelineReferencesSecret("secret-1", "default", &pipeline))
	require.True(t, LogPipelineReferencesSecret("secret-2", "default", &pipeline))
	require.True(t, LogPipelineReferencesSecret("creds", "default", &pipeline))

	require.False(t, LogPipelineReferencesSecret("secret-1", "kube-system", &pipeline))
	require.False(t, LogPipelineReferencesSecret("unknown", "default", &pipeline))
}
