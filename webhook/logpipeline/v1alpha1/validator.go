package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
	webhookutils "github.com/kyma-project/telemetry-manager/webhook/utils"
)

const (
	migrationGuideLink = "https://kyma-project.io/#/telemetry-manager/docs/user/integrate-otlp-backend/migration-to-otlp-logs.html"
)

type LogPipelineValidator struct {
}

var _ webhook.CustomValidator = &LogPipelineValidator{}

func (v *LogPipelineValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return validate(ctx, obj)
}

func (v *LogPipelineValidator) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	return validate(ctx, newObj)
}

func (v *LogPipelineValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validateFilterTransform(ctx context.Context, filterSpec []telemetryv1beta1.FilterSpec, transformSpec []telemetryv1beta1.TransformSpec) error {
	err := webhookutils.ValidateFilterTransform(ctx, ottl.SignalTypeLog, filterSpec, transformSpec)
	if err != nil {
		return fmt.Errorf(conditions.MessageForOtelLogPipeline(conditions.ReasonOTTLSpecInvalid), err.Error())
	}

	return nil
}

func validate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	pipeline, ok := obj.(*telemetryv1alpha1.LogPipeline)

	var warnings admission.Warnings

	if !ok {
		return nil, fmt.Errorf("expected a LogPipeline but got %T", obj)
	}

	filterSpec, transformSpec, err := webhookutils.ConvertFilterTransformToBeta(pipeline.Spec.Filters, pipeline.Spec.Transforms)
	if err != nil {
		return nil, err
	}

	if err := validateFilterTransform(ctx, filterSpec, transformSpec); err != nil {
		return nil, err
	}

	if isCustomFilterDefined(pipeline.Spec.FluentBitFilters) {
		warnings = append(warnings, renderDeprecationWarning(pipeline.Name, "filters"))
	}

	if isCustomOutputDefined(&pipeline.Spec.Output) {
		warnings = append(warnings, renderDeprecationWarning(pipeline.Name, "output.custom"))
	}

	if isHTTPOutputDefined(&pipeline.Spec.Output) {
		warnings = append(warnings, renderDeprecationWarning(pipeline.Name, "output.http"))
	}

	if isVariablesDefined(pipeline.Spec.FluentBitVariables) {
		warnings = append(warnings, renderDeprecationWarning(pipeline.Name, "variables"))
	}

	if isFilesDefined(pipeline.Spec.FluentBitFiles) {
		warnings = append(warnings, renderDeprecationWarning(pipeline.Name, "files"))
	}

	if isApplicationInputEnabled(&pipeline.Spec.Input) && pipeline.Spec.Input.Application.FluentBitDropLabels != nil {
		warnings = append(warnings, renderDeprecationWarning(pipeline.Name, "input.application.dropLabels"))
	}

	if isApplicationInputEnabled(&pipeline.Spec.Input) && pipeline.Spec.Input.Application.FluentBitKeepAnnotations != nil {
		warnings = append(warnings, renderDeprecationWarning(pipeline.Name, "input.application.keepAnnotations"))
	}

	return warnings, nil
}

func renderDeprecationWarning(pipelineName string, attribute string) string {
	return fmt.Sprintf("LogPipeline '%s' uses the attribute '%s' which is based on the deprecated FluentBit technology stack. Please migrate to an Open Telemetry based pipeline instead. See the documentation: %s", pipelineName, attribute, migrationGuideLink)
}

func isCustomFilterDefined(filters []telemetryv1alpha1.FluentBitFilter) bool {
	for _, filter := range filters {
		if filter.Custom != "" {
			return true
		}
	}

	return false
}

func isCustomOutputDefined(o *telemetryv1alpha1.LogPipelineOutput) bool {
	return o.FluentBitCustom != ""
}

func isHTTPOutputDefined(o *telemetryv1alpha1.LogPipelineOutput) bool {
	return o.FluentBitHTTP != nil
}

func isVariablesDefined(v []telemetryv1alpha1.FluentBitVariable) bool {
	return len(v) > 0
}

func isFilesDefined(v []telemetryv1alpha1.FluentBitFile) bool {
	return len(v) > 0
}

func isApplicationInputEnabled(i *telemetryv1alpha1.LogPipelineInput) bool {
	return i.Application != nil && ptr.Deref(i.Application.Enabled, false)
}
