package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestContainsNoOutputPlugins(t *testing.T) {
	logPipeline := &LogPipeline{
		Spec: LogPipelineSpec{
			Output: Output{},
		}}

	result := logPipeline.validateOutput()

	require.Error(t, result)
	require.Contains(t, result.Error(), "no output plugin is defined, you must define one output plugin")
}

func TestContainsMultipleOutputPlugins(t *testing.T) {
	logPipeline := &LogPipeline{
		Spec: LogPipelineSpec{
			Output: Output{
				Custom: `Name	http`,
				HTTP: &HTTPOutput{
					Host: ValueType{
						Value: "localhost",
					},
				},
			},
		}}
	result := logPipeline.validateOutput()

	require.Error(t, result)
	require.Contains(t, result.Error(), "multiple output plugins are defined, you must define only one output")
}

func TestValidateCustomOutput(t *testing.T) {
	logPipeline := &LogPipeline{
		Spec: LogPipelineSpec{
			Output: Output{
				Custom: `
   name    http`,
			},
		},
	}

	err := logPipeline.validateOutput()
	require.NoError(t, err)
}

func TestValidateCustomHasForbiddenParameter(t *testing.T) {
	logPipeline := &LogPipeline{
		Spec: LogPipelineSpec{
			Output: Output{
				Custom: `
   name    http
	storage.total_limit_size 10G`,
			},
		},
	}

	err := logPipeline.validateOutput()
	require.Error(t, err)
}

func TestValidateCustomOutputsContainsNoName(t *testing.T) {
	logPipeline := &LogPipeline{
		Spec: LogPipelineSpec{
			Output: Output{
				Custom: `
	Regex   .*`,
			},
		},
	}

	err := logPipeline.validateOutput()

	require.Error(t, err)
	require.Contains(t, err.Error(), "configuration section must have name attribute")
}

func TestBothValueAndValueFromPresent(t *testing.T) {
	logPipeline := &LogPipeline{
		Spec: LogPipelineSpec{
			Output: Output{
				HTTP: &HTTPOutput{
					Host: ValueType{
						Value: "localhost",
						ValueFrom: &ValueFromSource{
							SecretKeyRef: &SecretKeyRef{
								Name:      "foo",
								Namespace: "foo-ns",
								Key:       "foo-key",
							},
						},
					},
				},
			},
		}}
	err := logPipeline.validateOutput()
	require.Error(t, err)
	require.Contains(t, err.Error(), "http output host must have either a value or secret key reference")
}

func TestValueFromSecretKeyRef(t *testing.T) {
	logPipeline := &LogPipeline{
		Spec: LogPipelineSpec{
			Output: Output{
				HTTP: &HTTPOutput{
					Host: ValueType{
						ValueFrom: &ValueFromSource{
							SecretKeyRef: &SecretKeyRef{
								Name:      "foo",
								Namespace: "foo-ns",
								Key:       "foo-key",
							},
						},
					},
				},
			},
		}}
	err := logPipeline.validateOutput()
	require.NoError(t, err)
}

func TestValidateCustomFilter(t *testing.T) {
	logPipeline := &LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: LogPipelineSpec{
			Output: Output{
				Custom: `
    Name    http`,
			},
		},
	}

	err := logPipeline.validateFilters()
	require.NoError(t, err)
}

func TestValidateCustomFiltersContainsNoName(t *testing.T) {
	logPipeline := &LogPipeline{
		Spec: LogPipelineSpec{
			Filters: []Filter{
				{Custom: `
    Match   *`,
				},
			},
		},
	}

	err := logPipeline.validateFilters()
	require.Error(t, err)
	require.Contains(t, err.Error(), "configuration section must have name attribute")
}

func TestValidateCustomFiltersContainsMatch(t *testing.T) {
	logPipeline := &LogPipeline{
		Spec: LogPipelineSpec{
			Filters: []Filter{
				{Custom: `
    Name    grep
    Match   *`,
				},
			},
		},
	}

	err := logPipeline.validateFilters()

	require.Error(t, err)
	require.Contains(t, err.Error(), "plugin 'grep' contains match condition. Match conditions are forbidden")
}

func TestDeniedFilterPlugins(t *testing.T) {
	logPipeline := &LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: LogPipelineSpec{
			Filters: []Filter{
				{Custom: `
    Name    kubernetes`,
				},
			},
		},
	}

	err := logPipeline.validateFilters()

	require.Error(t, err)
	require.Contains(t, err.Error(), "plugin 'kubernetes' is forbidden. ")
}

func TestValidateWithValidInputIncludes(t *testing.T) {
	logPipeline := &LogPipeline{
		Spec: LogPipelineSpec{
			Input: Input{
				Application: ApplicationInput{
					Namespaces: InputNamespaces{
						Include: []string{"namespace-1", "namespace-2"},
					},
					Containers: InputContainers{
						Include: []string{"container-1"},
					},
				},
			},
		}}

	err := logPipeline.validateInput()
	require.NoError(t, err)
}

func TestValidateWithValidInputExcludes(t *testing.T) {
	logPipeline := &LogPipeline{
		Spec: LogPipelineSpec{
			Input: Input{
				Application: ApplicationInput{
					Namespaces: InputNamespaces{
						Exclude: []string{"namespace-1", "namespace-2"},
					},
					Containers: InputContainers{
						Exclude: []string{"container-1"},
					},
				},
			},
		},
	}

	err := logPipeline.validateInput()
	require.NoError(t, err)
}

func TestValidateWithValidInputIncludeContainersSystemFlag(t *testing.T) {
	logPipeline := &LogPipeline{
		Spec: LogPipelineSpec{
			Input: Input{
				Application: ApplicationInput{
					Namespaces: InputNamespaces{
						System: true,
					},
					Containers: InputContainers{
						Include: []string{"container-1"},
					},
				},
			},
		},
	}

	err := logPipeline.validateInput()
	require.NoError(t, err)
}

func TestValidateWithValidInputExcludeContainersSystemFlag(t *testing.T) {
	logPipeline := &LogPipeline{
		Spec: LogPipelineSpec{
			Input: Input{
				Application: ApplicationInput{
					Namespaces: InputNamespaces{
						System: true,
					},
					Containers: InputContainers{
						Exclude: []string{"container-1"},
					},
				},
			},
		},
	}

	err := logPipeline.validateInput()
	require.NoError(t, err)
}

func TestValidateWithInvalidNamespaceSelectors(t *testing.T) {
	logPipeline := &LogPipeline{
		Spec: LogPipelineSpec{
			Input: Input{
				Application: ApplicationInput{
					Namespaces: InputNamespaces{
						Include: []string{"namespace-1", "namespace-2"},
						Exclude: []string{"namespace-3"},
					},
				},
			},
		},
	}

	err := logPipeline.validateInput()
	require.Error(t, err)
}

func TestValidateWithInvalidIncludeSystemFlag(t *testing.T) {
	logPipeline := &LogPipeline{
		Spec: LogPipelineSpec{
			Input: Input{
				Application: ApplicationInput{
					Namespaces: InputNamespaces{
						Include: []string{"namespace-1", "namespace-2"},
						System:  true,
					},
				},
			},
		},
	}

	err := logPipeline.validateInput()
	require.Error(t, err)
}

func TestValidateWithInvalidExcludeSystemFlag(t *testing.T) {
	logPipeline := &LogPipeline{
		Spec: LogPipelineSpec{
			Input: Input{
				Application: ApplicationInput{
					Namespaces: InputNamespaces{
						Exclude: []string{"namespace-3"},
						System:  true,
					},
				},
			},
		},
	}

	err := logPipeline.validateInput()
	require.Error(t, err)
}

func TestValidateWithInvalidContainerSelectors(t *testing.T) {
	logPipeline := &LogPipeline{
		Spec: LogPipelineSpec{
			Input: Input{
				Application: ApplicationInput{
					Containers: InputContainers{
						Include: []string{"container-1", "container-2"},
						Exclude: []string{"container-3"},
					},
				},
			},
		},
	}

	err := logPipeline.validateInput()
	require.Error(t, err)
}
