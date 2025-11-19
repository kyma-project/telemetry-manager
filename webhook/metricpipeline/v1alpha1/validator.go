package v1alpha1

import (
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
	"github.com/kyma-project/telemetry-manager/webhook/common"
	webhookutils "github.com/kyma-project/telemetry-manager/webhook/utils"
)

type MetricPipelineValidator = common.PipelineValidator[*telemetryv1alpha1.MetricPipeline]

var _ webhook.CustomValidator = &MetricPipelineValidator{}

func NewMetricPipelineValidator() *MetricPipelineValidator {
	return common.NewValidatingWebhook[*telemetryv1alpha1.MetricPipeline]().
		WithSignalType(ottl.SignalTypeMetric).
		WithFilterExtractor(func(pipeline *telemetryv1alpha1.MetricPipeline) []telemetryv1beta1.FilterSpec {
			filterSpec, _ := webhookutils.ConvertFilterTransformToBeta(pipeline.Spec.Filters, nil)
			return filterSpec
		}).
		WithTransformExtractor(func(pipeline *telemetryv1alpha1.MetricPipeline) []telemetryv1beta1.TransformSpec {
			_, transformSpec := webhookutils.ConvertFilterTransformToBeta(nil, pipeline.Spec.Transforms)
			return transformSpec
		}).
		Build()
}
