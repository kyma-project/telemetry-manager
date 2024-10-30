package tracepipeline

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

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
			actual := GetSecretRefs(&sut)
			require.ElementsMatch(t, test.expected, actual)
		})
	}
}
