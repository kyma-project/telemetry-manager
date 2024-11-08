package validation

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func TestContainsNoOutputPlugins(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.LogPipelineOutput{},
		}}

	result := validateOutput(logPipeline)

	require.Error(t, result)
	require.Contains(t, result.Error(), "no output plugin is defined, you must define one output plugin")
}

func TestContainsMultipleOutputPlugins(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.LogPipelineOutput{
				Custom: `Name	http`,
				HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{
					Host: telemetryv1alpha1.ValueType{
						Value: "localhost",
					},
				},
			},
		}}
	result := validateOutput(logPipeline)

	require.Error(t, result)
	require.Contains(t, result.Error(), "multiple output plugins are defined, you must define only one output")
}

func TestValidateCustomOutput(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.LogPipelineOutput{
				Custom: `
   name    http`,
			},
		},
	}

	err := validateOutput(logPipeline)
	require.NoError(t, err)
}

func TestValidateCustomHasForbiddenParameter(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.LogPipelineOutput{
				Custom: `
   name    http
	storage.total_limit_size 10G`,
			},
		},
	}

	err := validateOutput(logPipeline)
	require.Error(t, err)
}

func TestValidateCustomOutputsContainsNoName(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.LogPipelineOutput{
				Custom: `
	Regex   .*`,
			},
		},
	}

	err := validateOutput(logPipeline)

	require.Error(t, err)
	require.Contains(t, err.Error(), "configuration section must have name attribute")
}

func TestBothValueAndValueFromPresent(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.LogPipelineOutput{
				HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{
					Host: telemetryv1alpha1.ValueType{
						Value: "localhost",
						ValueFrom: &telemetryv1alpha1.ValueFromSource{
							SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
								Name:      "foo",
								Namespace: "foo-ns",
								Key:       "foo-key",
							},
						},
					},
				},
			},
		}}
	err := validateOutput(logPipeline)
	require.Error(t, err)
	require.Contains(t, err.Error(), "http output host must have either a value or secret key reference")
}

func TestValueFromSecretKeyRef(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.LogPipelineOutput{
				HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{
					Host: telemetryv1alpha1.ValueType{
						ValueFrom: &telemetryv1alpha1.ValueFromSource{
							SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
								Name:      "foo",
								Namespace: "foo-ns",
								Key:       "foo-key",
							},
						},
					},
				},
			},
		}}
	err := validateOutput(logPipeline)
	require.NoError(t, err)
}

func TestValidateCustomFilter(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.LogPipelineOutput{
				Custom: `
    Name    http`,
			},
		},
	}

	err := validateFilters(logPipeline)
	require.NoError(t, err)
}

func TestValidateCustomFiltersContainsNoName(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Filters: []telemetryv1alpha1.LogPipelineFilter{
				{Custom: `
    Match   *`,
				},
			},
		},
	}

	err := validateFilters(logPipeline)
	require.Error(t, err)
	require.Contains(t, err.Error(), "configuration section must have name attribute")
}

func TestValidateCustomFiltersContainsMatch(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Filters: []telemetryv1alpha1.LogPipelineFilter{
				{Custom: `
    Name    grep
    Match   *`,
				},
			},
		},
	}

	err := validateFilters(logPipeline)

	require.Error(t, err)
	require.Contains(t, err.Error(), "plugin 'grep' contains match condition. Match conditions are forbidden")
}

func TestDeniedFilterPlugins(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Filters: []telemetryv1alpha1.LogPipelineFilter{
				{Custom: `
    Name    kubernetes`,
				},
			},
		},
	}

	err := validateFilters(logPipeline)

	require.Error(t, err)
	require.Contains(t, err.Error(), "plugin 'kubernetes' is forbidden. ")
}

func TestValidateWithValidInputIncludes(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.LogPipelineInput{
				Application: &telemetryv1alpha1.LogPipelineApplicationInput{
					Namespaces: telemetryv1alpha1.LogPipelineNamespaceSelector{
						Include: []string{"namespace-1", "namespace-2"},
					},
					Containers: telemetryv1alpha1.LogPipelineContainerSelector{
						Include: []string{"container-1"},
					},
				},
			},
		}}

	err := validateInput(logPipeline)
	require.NoError(t, err)
}

func TestValidateWithValidInputExcludes(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.LogPipelineInput{
				Application: &telemetryv1alpha1.LogPipelineApplicationInput{
					Namespaces: telemetryv1alpha1.LogPipelineNamespaceSelector{
						Exclude: []string{"namespace-1", "namespace-2"},
					},
					Containers: telemetryv1alpha1.LogPipelineContainerSelector{
						Exclude: []string{"container-1"},
					},
				},
			},
		},
	}

	err := validateInput(logPipeline)
	require.NoError(t, err)
}

func TestValidateWithValidInputIncludeContainersSystemFlag(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.LogPipelineInput{
				Application: &telemetryv1alpha1.LogPipelineApplicationInput{
					Namespaces: telemetryv1alpha1.LogPipelineNamespaceSelector{
						System: true,
					},
					Containers: telemetryv1alpha1.LogPipelineContainerSelector{
						Include: []string{"container-1"},
					},
				},
			},
		},
	}

	err := validateInput(logPipeline)
	require.NoError(t, err)
}

func TestValidateWithValidInputExcludeContainersSystemFlag(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.LogPipelineInput{
				Application: &telemetryv1alpha1.LogPipelineApplicationInput{
					Namespaces: telemetryv1alpha1.LogPipelineNamespaceSelector{
						System: true,
					},
					Containers: telemetryv1alpha1.LogPipelineContainerSelector{
						Exclude: []string{"container-1"},
					},
				},
			},
		},
	}

	err := validateInput(logPipeline)
	require.NoError(t, err)
}

func TestValidateWithInvalidNamespaceSelectors(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.LogPipelineInput{
				Application: &telemetryv1alpha1.LogPipelineApplicationInput{
					Namespaces: telemetryv1alpha1.LogPipelineNamespaceSelector{
						Include: []string{"namespace-1", "namespace-2"},
						Exclude: []string{"namespace-3"},
					},
				},
			},
		},
	}

	err := validateInput(logPipeline)
	require.Error(t, err)
}

func TestValidateWithInvalidIncludeSystemFlag(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.LogPipelineInput{
				Application: &telemetryv1alpha1.LogPipelineApplicationInput{
					Namespaces: telemetryv1alpha1.LogPipelineNamespaceSelector{
						Include: []string{"namespace-1", "namespace-2"},
						System:  true,
					},
				},
			},
		},
	}

	err := validateInput(logPipeline)
	require.Error(t, err)
}

func TestValidateWithInvalidExcludeSystemFlag(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.LogPipelineInput{
				Application: &telemetryv1alpha1.LogPipelineApplicationInput{
					Namespaces: telemetryv1alpha1.LogPipelineNamespaceSelector{
						Exclude: []string{"namespace-3"},
						System:  true,
					},
				},
			},
		},
	}

	err := validateInput(logPipeline)
	require.Error(t, err)
}

func TestValidateWithInvalidContainerSelectors(t *testing.T) {
	logPipeline := &telemetryv1alpha1.LogPipeline{
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Input: telemetryv1alpha1.LogPipelineInput{
				Application: &telemetryv1alpha1.LogPipelineApplicationInput{
					Containers: telemetryv1alpha1.LogPipelineContainerSelector{
						Include: []string{"container-1", "container-2"},
						Exclude: []string{"container-3"},
					},
				},
			},
		},
	}

	err := validateInput(logPipeline)
	require.Error(t, err)
}
