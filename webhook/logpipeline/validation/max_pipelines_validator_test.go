package validation

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func TestValidateWithoutLimit(t *testing.T) {
	validator := NewMaxPipelinesValidator(0)
	pipeline := telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipeline-1",
		},
	}
	pipelines := telemetryv1alpha1.LogPipelineList{}

	err := validator.Validate(&pipeline, &pipelines)
	require.NoError(t, err)
}

func TestValidateFirstPipeline(t *testing.T) {
	validator := NewMaxPipelinesValidator(1)
	pipeline := telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipeline-1",
		},
	}
	pipelines := telemetryv1alpha1.LogPipelineList{}

	err := validator.Validate(&pipeline, &pipelines)
	require.NoError(t, err)
}

func TestValidateUpdate(t *testing.T) {
	validator := NewMaxPipelinesValidator(1)
	pipeline := telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipeline-1",
		},
	}
	pipelines := telemetryv1alpha1.LogPipelineList{}
	pipelines.Items = append(pipelines.Items, pipeline)

	err := validator.Validate(&pipeline, &pipelines)
	require.NoError(t, err)
}

func TestValidateSecondPipeline(t *testing.T) {
	validator := NewMaxPipelinesValidator(2)
	pipeline1 := telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipeline-1",
		},
	}
	pipeline2 := telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipeline-2",
		},
	}

	pipelines := telemetryv1alpha1.LogPipelineList{}
	pipelines.Items = append(pipelines.Items, pipeline1)

	err := validator.Validate(&pipeline2, &pipelines)
	require.NoError(t, err)
}

func TestValidateLimitExceeded(t *testing.T) {
	validator := NewMaxPipelinesValidator(1)
	pipeline1 := telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipeline-1",
		},
	}
	pipeline2 := telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipeline-2",
		},
	}

	pipelines := telemetryv1alpha1.LogPipelineList{}
	pipelines.Items = append(pipelines.Items, pipeline1)

	err := validator.Validate(&pipeline2, &pipelines)
	require.Error(t, err)
}

func TestIsNewPipeline(t *testing.T) {
	var validator maxPipelinesValidator
	pipeline1 := telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipeline-1",
		},
	}
	pipeline2 := telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipeline-2",
		},
	}

	pipelines := telemetryv1alpha1.LogPipelineList{}
	pipelines.Items = append(pipelines.Items, pipeline1)

	require.True(t, validator.isNewPipeline(&pipeline2, &pipelines))
	require.False(t, validator.isNewPipeline(&pipeline1, &pipelines))
}
