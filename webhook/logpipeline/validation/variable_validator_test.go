package validation

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/k8sutils/mocks"
)

func TestValidateSecretKeyRefs(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Variables: []telemetryv1alpha1.VariableRef{
				{
					Name: "foo1",
					ValueFrom: telemetryv1alpha1.ValueFromSource{
						SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
							Name:      "fooN",
							Namespace: "fooNs",
							Key:       "foo",
						},
					},
				},
				{
					Name: "foo2",
					ValueFrom: telemetryv1alpha1.ValueFromSource{
						SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
							Name:      "fooN",
							Namespace: "fooNs",
							Key:       "foo",
						}},
				},
			},
		},
	}
	logPipeline.Name = "pipe1"
	logPipelines := &telemetryv1alpha1.LogPipelineList{
		Items: []telemetryv1alpha1.LogPipeline{*logPipeline},
	}

	newLogPipeline := &telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipe2",
		},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Variables: []telemetryv1alpha1.VariableRef{{
				Name: "foo2",
				ValueFrom: telemetryv1alpha1.ValueFromSource{
					SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
						Name:      "fooN",
						Namespace: "fooNs",
						Key:       "foo",
					}},
			}},
		},
	}
	mockClient := &mocks.Client{}
	varValidator := NewVariablesValidator(mockClient)

	err := varValidator.Validate(newLogPipeline, logPipelines)
	require.Error(t, err)
}

func TestVariableValidator(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Variables: []telemetryv1alpha1.VariableRef{
				{
					Name: "foo1",
					ValueFrom: telemetryv1alpha1.ValueFromSource{
						SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
							Name:      "fooN",
							Namespace: "fooNs",
							Key:       "foo",
						},
					},
				},
				{
					Name: "foo2",
					ValueFrom: telemetryv1alpha1.ValueFromSource{
						SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
							Name:      "",
							Namespace: "",
							Key:       "",
						}},
				},
			},
		},
	}
	logPipeline.Name = "pipe1"
	mockClient := &mocks.Client{}
	varValidator := NewVariablesValidator(mockClient)
	logPipelines := &telemetryv1alpha1.LogPipelineList{
		Items: []telemetryv1alpha1.LogPipeline{*logPipeline},
	}

	err := varValidator.Validate(logPipeline, logPipelines)
	require.Error(t, err)
	require.Equal(t, "mandatory field variable name or secretKeyRef name or secretKeyRef namespace or secretKeyRef key cannot be empty", err.Error())
}
