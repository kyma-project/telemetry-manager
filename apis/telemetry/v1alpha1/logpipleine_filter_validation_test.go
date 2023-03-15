package v1alpha1

import (
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

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
	err := logPipeline.ValidateFilters(vc.DeniedFilterPlugins)
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
	err := logPipeline.ValidateFilters(vc.DeniedFilterPlugins)
	require.Error(t, err)
	require.Contains(t, err.Error(), "configuration section does not have name attribute")
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
	err := logPipeline.ValidateFilters(vc.DeniedFilterPlugins)

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
	err := logPipeline.ValidateFilters(vc.DeniedFilterPlugins)

	require.Error(t, err)
	require.Contains(t, err.Error(), "plugin 'lua' is forbidden. ")
}
