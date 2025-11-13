package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
)

type LogPipelineValidator struct {
}

var _ webhook.CustomValidator = &LogPipelineValidator{}

func (v *LogPipelineValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	logPipeline, ok := obj.(*telemetryv1alpha1.LogPipeline)
	var warnings admission.Warnings

	if !ok {
		return nil, fmt.Errorf("expected a LogPipeline but got %T", obj)
	}

	if err := validateFilterTransform(ottl.SignalTypeLog, logPipeline.Spec.Filters, logPipeline.Spec.Transforms); err != nil {
		return nil, err
	}

	if logpipelineutils.ContainsCustomPlugin(logPipeline) {
		helpText := "https://kyma-project.io/#/telemetry-manager/user/02-logs"
		msg := fmt.Sprintf("Logpipeline '%s' uses unsupported custom filters or outputs. We recommend changing the pipeline to use supported filters or output. See the documentation: %s", logPipeline.Name, helpText)
		warnings = append(warnings, msg)

		return warnings, nil
	}
	return nil, nil
}

func (v *LogPipelineValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	logPipeline, ok := newObj.(*telemetryv1alpha1.LogPipeline)
	var warnings admission.Warnings

	if !ok {
		return nil, fmt.Errorf("expected a LogPipeline but got %T", newObj)
	}

	if err := validateFilterTransform(ottl.SignalTypeLog, logPipeline.Spec.Filters, logPipeline.Spec.Transforms); err != nil {
		return nil, err
	}

	if logpipelineutils.ContainsCustomPlugin(logPipeline) {
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

func validateFilterTransform(signalType ottl.SignalType, filterSpec []telemetryv1alpha1.FilterSpec, transformSpec []telemetryv1alpha1.TransformSpec) error {
	filterValidator, err := ottl.NewFilterSpecValidator(signalType)
	if err != nil {
		return fmt.Errorf("failed to instantiate FilterSpecValidator %w", err)
	}

	err = filterValidator.Validate(filterSpec)
	if err != nil {
		return err
	}

	transformValidator, err := ottl.NewTransformSpecValidator(signalType)
	if err != nil {
		return fmt.Errorf("failed to instantiate TransformSpecValidator %w", err)
	}

	err = transformValidator.Validate(transformSpec)
	if err != nil {
		return err
	}

	return nil
}
