package logpipeline

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

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
			actual := GetSecretRefs(&test.given)
			require.ElementsMatch(t, test.expected, actual)
		})
	}
}
