package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/pipelines"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
	webhookutils "github.com/kyma-project/telemetry-manager/webhook/utils"
)

const (
	migrationGuideLink = "https://kyma-project.io/#/telemetry-manager/user/integrate-otlp-backend/migration-to-otlp-logs"
)

type validator struct {
	secretAccessValidator *secretref.AccessValidator
}

func newValidator(c client.Client) *validator {
	return &validator{secretAccessValidator: &secretref.AccessValidator{Client: c}}
}

var _ admission.Validator[*telemetryv1alpha1.LogPipeline] = &validator{}

func (v *validator) ValidateCreate(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) (admission.Warnings, error) {
	return v.validate(ctx, pipeline)
}

func (v *validator) ValidateUpdate(ctx context.Context, _, newPipeline *telemetryv1alpha1.LogPipeline) (admission.Warnings, error) {
	return v.validate(ctx, newPipeline)
}

func (v *validator) ValidateDelete(_ context.Context, _ *telemetryv1alpha1.LogPipeline) (admission.Warnings, error) {
	return nil, nil
}

func (v *validator) validate(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) (admission.Warnings, error) {
	var warnings admission.Warnings

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

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get admission request from context: %w", err)
	}

	var converted telemetryv1beta1.LogPipeline
	if err := telemetryv1alpha1.Convert_v1alpha1_LogPipeline_To_v1beta1_LogPipeline(pipeline, &converted, nil); err != nil {
		return nil, fmt.Errorf("failed to convert LogPipeline to v1beta1: %w", err)
	}

	refs := secretref.GetSecretRefsLogPipeline(&converted)
	if err := v.secretAccessValidator.ValidateSecretAccess(ctx, req.UserInfo, refs); err != nil {
		return nil, err
	}

	return warnings, nil
}

func validateFilterTransform(ctx context.Context, filterSpec []telemetryv1beta1.FilterSpec, transformSpec []telemetryv1beta1.TransformSpec) error {
	err := webhookutils.ValidateFilterTransform(ctx, pipelines.SignalTypeLog, filterSpec, transformSpec)
	if err != nil {
		return fmt.Errorf(conditions.MessageForOtelLogPipeline(conditions.ReasonOTTLSpecInvalid), err.Error())
	}

	return nil
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
