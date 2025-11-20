package v1beta1

import (
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/webhook/common"
)

type MetricPipelineValidator = common.PipelineValidator[*telemetryv1beta1.MetricPipeline]

var _ webhook.CustomValidator = &MetricPipelineValidator{}

func NewMetricPipelineValidator() *MetricPipelineValidator {
	return common.NewPipelineValidator[*telemetryv1beta1.MetricPipeline]().
		WithFilterExtractor(func(pipeline *telemetryv1beta1.MetricPipeline) []telemetryv1beta1.FilterSpec {
			return pipeline.Spec.Filters
		}).
		WithTransformExtractor(func(pipeline *telemetryv1beta1.MetricPipeline) []telemetryv1beta1.TransformSpec {
			return pipeline.Spec.Transforms
		}).
		Build()
}
