package v1beta1

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/webhook/common"
)

type LogPipelineValidator = common.PipelineValidator[*telemetryv1beta1.LogPipeline]

var _ webhook.CustomValidator = &LogPipelineValidator{}

func NewLogPipelineValidator() *LogPipelineValidator {
	return common.NewPipelineValidator[*telemetryv1beta1.LogPipeline]().
		WithFilterExtractor(func(pipeline *telemetryv1beta1.LogPipeline) []telemetryv1beta1.FilterSpec {
			return pipeline.Spec.Filters
		}).
		WithTransformExtractor(func(pipeline *telemetryv1beta1.LogPipeline) []telemetryv1beta1.TransformSpec {
			return pipeline.Spec.Transforms
		}).
		WithCustomWarningCheck(func(pipeline *telemetryv1beta1.LogPipeline) (admission.Warnings, bool) {
			if containsCustomPlugin(pipeline) {
				helpText := "https://kyma-project.io/#/telemetry-manager/user/02-logs"
				msg := fmt.Sprintf("Logpipeline '%s' uses unsupported custom filters or outputs. We recommend changing the pipeline to use supported filters or output. See the documentation: %s", pipeline.Name, helpText)

				return admission.Warnings{msg}, true
			}

			return nil, false
		}).
		Build()
}

// containsCustomPlugin returns true if the pipeline contains any custom filters or outputs
func containsCustomPlugin(lp *telemetryv1beta1.LogPipeline) bool {
	for _, filter := range lp.Spec.FluentBitFilters {
		if filter.Custom != "" {
			return true
		}
	}

	return lp.Spec.Output.Custom != ""
}
