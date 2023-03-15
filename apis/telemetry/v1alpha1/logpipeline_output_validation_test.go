package v1alpha1

import (
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestContainsNoOutputPlugins(t *testing.T) {
	logPipeline := &LogPipeline{
		Spec: LogPipelineSpec{
			Output: Output{},
		}}

	vc := getLogPipelineValidationConfig()
	result := logPipeline.ValidateOutput(vc.DeniedOutPutPlugins)

	require.Error(t, result)
	require.Contains(t, result.Error(), "no output is defined, you must define one output")
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
	result := logPipeline.ValidateOutput(vc.DeniedOutPutPlugins)

	require.Error(t, result)
	require.Contains(t, result.Error(), "multiple output plugins are defined, you must define only one output")
}

func TestDeniedOutputPlugins(t *testing.T) {
	logPipeline := &LogPipeline{
		ObjectMeta: v1.ObjectMeta{Name: "foo"},
		Spec: LogPipelineSpec{
			Output: Output{
				Custom: `
   Name    lua`,
			},
		},
	}

	vc := getLogPipelineValidationConfig()
	err := logPipeline.ValidateOutput(vc.DeniedOutPutPlugins)

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
	err := logPipeline.ValidateOutput(vc.DeniedOutPutPlugins)
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
	err := logPipeline.ValidateOutput(vc.DeniedOutPutPlugins)
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
	err := logPipeline.ValidateOutput(vc.DeniedOutPutPlugins)

	require.Error(t, err)
	require.Contains(t, err.Error(), "configuration section does not have name attribute")
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
	err := logPipeline.ValidateOutput(vc.DeniedOutPutPlugins)
	require.Error(t, err)
	require.Contains(t, err.Error(), "http output host needs to have either value or secret key reference")
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
	err := logPipeline.ValidateOutput(vc.DeniedOutPutPlugins)
	require.NoError(t, err)
}

func getLogPipelineValidationConfig() LogPipelineValidationConfig {
	return LogPipelineValidationConfig{DeniedOutPutPlugins: []string{"lua", "multiline"}, DeniedFilterPlugins: []string{"lua", "multiline"}}
}
