package v1beta1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
	webhookutils "github.com/kyma-project/telemetry-manager/webhook/utils"
)

type LogPipelineValidator struct {
}

var _ webhook.CustomValidator = &LogPipelineValidator{}

func (v *LogPipelineValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	logPipeline, ok := obj.(*telemetryv1beta1.LogPipeline)

	var warnings admission.Warnings

	if !ok {
		return nil, fmt.Errorf("expected a LogPipeline but got %T", obj)
	}

	if err := validateFilterTransform(logPipeline.Spec.Filters, logPipeline.Spec.Transforms); err != nil {
		return nil, err
	}

	if containsCustomPlugin(logPipeline) {
		helpText := "https://kyma-project.io/#/telemetry-manager/user/02-logs"
		msg := fmt.Sprintf("Logpipeline '%s' uses unsupported custom filters or outputs. We recommend changing the pipeline to use supported filters or output. See the documentation: %s", logPipeline.Name, helpText)
		warnings = append(warnings, msg)

		return warnings, nil
	}

	return nil, nil
}

func (v *LogPipelineValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	logPipeline, ok := newObj.(*telemetryv1beta1.LogPipeline)

	var warnings admission.Warnings

	if !ok {
		return nil, fmt.Errorf("expected a LogPipeline but got %T", newObj)
	}

	if err := validateFilterTransform(logPipeline.Spec.Filters, logPipeline.Spec.Transforms); err != nil {
		return nil, err
	}

	if containsCustomPlugin(logPipeline) {
		helpText := "https://kyma-project.io/#/telemetry-manager/user/02-logs"
		msg := fmt.Sprintf("Logpipeline '%s' uses unsupported custom filters or outputs. We recommend changing the pipeline to use supported filters or output. See the documentation: %s", logPipeline.Name, helpText)
		warnings = append(warnings, msg)

		return warnings, nil
	}

	return nil, nil
}

func (v *LogPipelineValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ContainsCustomPlugin returns true if the pipeline contains any custom filters or outputs
func containsCustomPlugin(lp *telemetryv1beta1.LogPipeline) bool {
	for _, filter := range lp.Spec.FluentBitFilters {
		if filter.Custom != "" {
			return true
		}
	}

	return lp.Spec.Output.Custom != ""
}

func validateFilterTransform(filterSpec []telemetryv1beta1.FilterSpec, transformSpec []telemetryv1beta1.TransformSpec) error {
	err := webhookutils.ValidateFilterTransform(ottl.SignalTypeLog, filterSpec, transformSpec)
	if err != nil {
		return fmt.Errorf(conditions.MessageForOtelLogPipeline(conditions.ReasonOTTLSpecInvalid), err.Error())
	}

	return nil
}
