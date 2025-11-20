package common_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/webhook/common"
)

func TestPipelineValidator_ValidateCreate_ValidFilter(t *testing.T) {
	validator := common.NewPipelineValidator[*telemetryv1beta1.MetricPipeline]().
		WithFilterExtractor(func(pipeline *telemetryv1beta1.MetricPipeline) []telemetryv1beta1.FilterSpec {
			return pipeline.Spec.Filters
		}).
		WithTransformExtractor(func(pipeline *telemetryv1beta1.MetricPipeline) []telemetryv1beta1.TransformSpec {
			return pipeline.Spec.Transforms
		}).
		Build()

	pipeline := &telemetryv1beta1.MetricPipeline{
		Spec: telemetryv1beta1.MetricPipelineSpec{
			Filters: []telemetryv1beta1.FilterSpec{
				{
					Conditions: []string{`IsMatch(metric.name, "envoy") == true`},
				},
			},
		},
	}

	warnings, err := validator.ValidateCreate(context.Background(), pipeline)

	require.NoError(t, err)
	require.Empty(t, warnings)
}

func TestPipelineValidator_ValidateCreate_InvalidFilter(t *testing.T) {
	validator := common.NewPipelineValidator[*telemetryv1beta1.MetricPipeline]().
		WithFilterExtractor(func(pipeline *telemetryv1beta1.MetricPipeline) []telemetryv1beta1.FilterSpec {
			return pipeline.Spec.Filters
		}).
		WithTransformExtractor(func(pipeline *telemetryv1beta1.MetricPipeline) []telemetryv1beta1.TransformSpec {
			return pipeline.Spec.Transforms
		}).
		Build()

	pipeline := &telemetryv1beta1.MetricPipeline{
		Spec: telemetryv1beta1.MetricPipelineSpec{
			Filters: []telemetryv1beta1.FilterSpec{
				{
					Conditions: []string{`IsMatch(metric.name, "envoy") ?= true`}, // Invalid syntax
				},
			},
		},
	}

	warnings, err := validator.ValidateCreate(context.Background(), pipeline)

	require.Error(t, err)
	require.Empty(t, warnings)
}

func TestPipelineValidator_ValidateCreate_EmptyFields(t *testing.T) {
	validator := common.NewPipelineValidator[*telemetryv1beta1.MetricPipeline]().
		WithFilterExtractor(func(pipeline *telemetryv1beta1.MetricPipeline) []telemetryv1beta1.FilterSpec {
			return pipeline.Spec.Filters
		}).
		WithTransformExtractor(func(pipeline *telemetryv1beta1.MetricPipeline) []telemetryv1beta1.TransformSpec {
			return pipeline.Spec.Transforms
		}).
		Build()

	pipeline := &telemetryv1beta1.MetricPipeline{
		Spec: telemetryv1beta1.MetricPipelineSpec{
			Filters:    []telemetryv1beta1.FilterSpec{},
			Transforms: []telemetryv1beta1.TransformSpec{},
		},
	}

	warnings, err := validator.ValidateCreate(context.Background(), pipeline)

	require.NoError(t, err)
	require.Empty(t, warnings)
}

func TestPipelineValidator_ValidateUpdate_ValidUpdate(t *testing.T) {
	validator := common.NewPipelineValidator[*telemetryv1beta1.MetricPipeline]().
		WithFilterExtractor(func(pipeline *telemetryv1beta1.MetricPipeline) []telemetryv1beta1.FilterSpec {
			return pipeline.Spec.Filters
		}).
		WithTransformExtractor(func(pipeline *telemetryv1beta1.MetricPipeline) []telemetryv1beta1.TransformSpec {
			return pipeline.Spec.Transforms
		}).
		Build()

	oldPipeline := &telemetryv1beta1.MetricPipeline{
		Spec: telemetryv1beta1.MetricPipelineSpec{
			Filters: []telemetryv1beta1.FilterSpec{},
		},
	}

	newPipeline := &telemetryv1beta1.MetricPipeline{
		Spec: telemetryv1beta1.MetricPipelineSpec{
			Filters: []telemetryv1beta1.FilterSpec{
				{
					Conditions: []string{`IsMatch(metric.name, "envoy") == true`},
				},
			},
		},
	}

	warnings, err := validator.ValidateUpdate(context.Background(), oldPipeline, newPipeline)

	require.NoError(t, err)
	require.Empty(t, warnings)
}

func TestPipelineValidator_ValidateUpdate_InvalidUpdate(t *testing.T) {
	validator := common.NewPipelineValidator[*telemetryv1beta1.MetricPipeline]().
		WithFilterExtractor(func(pipeline *telemetryv1beta1.MetricPipeline) []telemetryv1beta1.FilterSpec {
			return pipeline.Spec.Filters
		}).
		WithTransformExtractor(func(pipeline *telemetryv1beta1.MetricPipeline) []telemetryv1beta1.TransformSpec {
			return pipeline.Spec.Transforms
		}).
		Build()

	oldPipeline := &telemetryv1beta1.MetricPipeline{
		Spec: telemetryv1beta1.MetricPipelineSpec{
			Filters: []telemetryv1beta1.FilterSpec{},
		},
	}

	newPipeline := &telemetryv1beta1.MetricPipeline{
		Spec: telemetryv1beta1.MetricPipelineSpec{
			Filters: []telemetryv1beta1.FilterSpec{
				{
					Conditions: []string{`log.severity_number <? SEVERITY_NUMBER_WARN`}, // Invalid syntax
				},
			},
		},
	}

	warnings, err := validator.ValidateUpdate(context.Background(), oldPipeline, newPipeline)

	require.Error(t, err)
	require.Empty(t, warnings)
}

func TestPipelineValidator_ValidateDelete(t *testing.T) {
	validator := common.NewPipelineValidator[*telemetryv1beta1.MetricPipeline]().
		WithFilterExtractor(func(pipeline *telemetryv1beta1.MetricPipeline) []telemetryv1beta1.FilterSpec {
			return pipeline.Spec.Filters
		}).
		WithTransformExtractor(func(pipeline *telemetryv1beta1.MetricPipeline) []telemetryv1beta1.TransformSpec {
			return pipeline.Spec.Transforms
		}).
		Build()

	pipeline := &telemetryv1beta1.MetricPipeline{}

	warnings, err := validator.ValidateDelete(context.Background(), pipeline)

	require.NoError(t, err)
	require.Empty(t, warnings)
}

func TestPipelineValidator_WrongType(t *testing.T) {
	validator := common.NewPipelineValidator[*telemetryv1beta1.MetricPipeline]().
		WithFilterExtractor(func(pipeline *telemetryv1beta1.MetricPipeline) []telemetryv1beta1.FilterSpec {
			return pipeline.Spec.Filters
		}).
		WithTransformExtractor(func(pipeline *telemetryv1beta1.MetricPipeline) []telemetryv1beta1.TransformSpec {
			return pipeline.Spec.Transforms
		}).
		Build()

	wrongObject := &telemetryv1beta1.TracePipeline{}

	warnings, err := validator.ValidateCreate(context.Background(), wrongObject)

	assert.ErrorContains(t, err, "expected a *v1beta1.MetricPipeline but got")
	assert.Empty(t, warnings)
}

func TestPipelineValidator_WithCustomWarningCheck(t *testing.T) {
	validator := common.NewPipelineValidator[*telemetryv1beta1.MetricPipeline]().
		WithFilterExtractor(func(pipeline *telemetryv1beta1.MetricPipeline) []telemetryv1beta1.FilterSpec {
			return pipeline.Spec.Filters
		}).
		WithTransformExtractor(func(pipeline *telemetryv1beta1.MetricPipeline) []telemetryv1beta1.TransformSpec {
			return pipeline.Spec.Transforms
		}).
		WithCustomWarningCheck(func(pipeline *telemetryv1beta1.MetricPipeline) (admission.Warnings, bool) {
			if pipeline.Name == "deprecated-pipeline" {
				return admission.Warnings{"This pipeline is deprecated"}, true
			}

			return nil, false
		}).
		Build()

	pipeline := &telemetryv1beta1.MetricPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "deprecated-pipeline",
		},
		Spec: telemetryv1beta1.MetricPipelineSpec{
			Filters: []telemetryv1beta1.FilterSpec{},
		},
	}

	warnings, err := validator.ValidateCreate(context.Background(), pipeline)

	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "deprecated")
}

func TestPipelineValidator_CustomWarningDoesNotBlockValidation(t *testing.T) {
	validator := common.NewPipelineValidator[*telemetryv1beta1.MetricPipeline]().
		WithFilterExtractor(func(pipeline *telemetryv1beta1.MetricPipeline) []telemetryv1beta1.FilterSpec {
			return pipeline.Spec.Filters
		}).
		WithTransformExtractor(func(pipeline *telemetryv1beta1.MetricPipeline) []telemetryv1beta1.TransformSpec {
			return pipeline.Spec.Transforms
		}).
		WithCustomWarningCheck(func(pipeline *telemetryv1beta1.MetricPipeline) (admission.Warnings, bool) {
			// Return warning but don't block validation
			return admission.Warnings{"Warning: this is a test"}, false
		}).
		Build()

	pipeline := &telemetryv1beta1.MetricPipeline{
		Spec: telemetryv1beta1.MetricPipelineSpec{
			Filters: []telemetryv1beta1.FilterSpec{
				{
					Conditions: []string{`invalid syntax ?=`},
				},
			},
		},
	}

	warnings, err := validator.ValidateCreate(context.Background(), pipeline)

	require.Error(t, err)
	require.Empty(t, warnings)
}
