package v1alpha1

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
	"github.com/kyma-project/telemetry-manager/webhook/common"
	webhookutils "github.com/kyma-project/telemetry-manager/webhook/utils"
)

type LogPipelineValidator = common.PipelineValidator[*telemetryv1alpha1.LogPipeline]

var _ webhook.CustomValidator = &LogPipelineValidator{}

func NewLogPipelineValidator() *LogPipelineValidator {
	return common.NewValidatingWebhook[*telemetryv1alpha1.LogPipeline]().
		WithSignalType(ottl.SignalTypeLog).
		WithFilterExtractor(func(pipeline *telemetryv1alpha1.LogPipeline) []telemetryv1beta1.FilterSpec {
			filterSpec, _ := webhookutils.ConvertFilterTransformToBeta(pipeline.Spec.Filters, nil)
			return filterSpec
		}).
		WithTransformExtractor(func(pipeline *telemetryv1alpha1.LogPipeline) []telemetryv1beta1.TransformSpec {
			_, transformSpec := webhookutils.ConvertFilterTransformToBeta(nil, pipeline.Spec.Transforms)
			return transformSpec
		}).
		WithCustomWarningCheck(func(pipeline *telemetryv1alpha1.LogPipeline) (admission.Warnings, bool) {
			if logpipelineutils.ContainsCustomPlugin(pipeline) {
				helpText := "https://kyma-project.io/#/telemetry-manager/user/02-logs"
				msg := fmt.Sprintf("Logpipeline '%s' uses unsupported custom filters or outputs. We recommend changing the pipeline to use supported filters or output. See the documentation: %s", pipeline.Name, helpText)
				return admission.Warnings{msg}, true
			}
			return nil, false
		}).
		Build()
}
