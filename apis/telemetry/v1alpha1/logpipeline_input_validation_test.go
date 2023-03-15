package v1alpha1

import (
	"github.com/stretchr/testify/require"
	"testing"
)

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

	err := logPipeline.ValidateInput()
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

	err := logPipeline.ValidateInput()
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

	err := logPipeline.ValidateInput()
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

	err := logPipeline.ValidateInput()
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

	err := logPipeline.ValidateInput()
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

	err := logPipeline.ValidateInput()
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

	err := logPipeline.ValidateInput()
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

	err := logPipeline.ValidateInput()
	require.Error(t, err)
}
