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

	vc := getLogPipelineValidationConfig()
	result := logPipeline.validateOutput(vc.DeniedOutPutPlugins)

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
	vc := getLogPipelineValidationConfig()
	result := logPipeline.validateOutput(vc.DeniedOutPutPlugins)

	require.Error(t, result)
	require.Contains(t, result.Error(), "multiple output plugins are defined, you must define only one output")
}

func TestDeniedOutputPlugins(t *testing.T) {
	logPipeline := &LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: LogPipelineSpec{
			Output: Output{
				Custom: `
   Name    lua`,
			},
		},
	}

	vc := getLogPipelineValidationConfig()
	err := logPipeline.validateOutput(vc.DeniedOutPutPlugins)

	require.Error(t, err)
	require.Contains(t, err.Error(), "plugin 'lua' is forbidden. ")
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

	vc := getLogPipelineValidationConfig()
	err := logPipeline.validateOutput(vc.DeniedOutPutPlugins)
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

	vc := getLogPipelineValidationConfig()
	err := logPipeline.validateOutput(vc.DeniedOutPutPlugins)
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

	vc := getLogPipelineValidationConfig()
	err := logPipeline.validateOutput(vc.DeniedOutPutPlugins)

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
	vc := getLogPipelineValidationConfig()
	err := logPipeline.validateOutput(vc.DeniedOutPutPlugins)
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
	vc := getLogPipelineValidationConfig()
	err := logPipeline.validateOutput(vc.DeniedOutPutPlugins)
	require.NoError(t, err)
}

func getLogPipelineValidationConfig() LogPipelineValidationConfig {
	return LogPipelineValidationConfig{DeniedOutPutPlugins: []string{"lua", "multiline"}, DeniedFilterPlugins: []string{"lua", "multiline"}}
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

	vc := getLogPipelineValidationConfig()
	err := logPipeline.validateFilters(vc.DeniedFilterPlugins)
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

	vc := getLogPipelineValidationConfig()
	err := logPipeline.validateFilters(vc.DeniedFilterPlugins)
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

	vc := getLogPipelineValidationConfig()
	err := logPipeline.validateFilters(vc.DeniedFilterPlugins)

	require.Error(t, err)
	require.Contains(t, err.Error(), "plugin 'grep' contains match condition. Match conditions are forbidden")
}

func TestDeniedFilterPlugins(t *testing.T) {
	logPipeline := &LogPipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: LogPipelineSpec{
			Filters: []Filter{
				{Custom: `
    Name    lua`,
				},
			},
		},
	}

	vc := getLogPipelineValidationConfig()
	err := logPipeline.validateFilters(vc.DeniedFilterPlugins)

	require.Error(t, err)
	require.Contains(t, err.Error(), "plugin 'lua' is forbidden. ")
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
