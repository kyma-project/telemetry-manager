package v1beta1

import (
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
	"github.com/kyma-project/telemetry-manager/webhook/common"
)

type TracePipelineValidator = common.PipelineValidator[*telemetryv1beta1.TracePipeline]

var _ webhook.CustomValidator = &TracePipelineValidator{}

func NewTracePipelineValidator() *TracePipelineValidator {
	return common.NewValidatingWebhook[*telemetryv1beta1.TracePipeline]().
		WithSignalType(ottl.SignalTypeTrace).
		WithFilterExtractor(func(pipeline *telemetryv1beta1.TracePipeline) []telemetryv1beta1.FilterSpec {
			return pipeline.Spec.Filters
		}).
		WithTransformExtractor(func(pipeline *telemetryv1beta1.TracePipeline) []telemetryv1beta1.TransformSpec {
			return pipeline.Spec.Transforms
		}).
		Build()
}
