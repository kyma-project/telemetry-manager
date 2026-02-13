package v1beta1

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
	webhookutils "github.com/kyma-project/telemetry-manager/webhook/utils"
)

const (
	migrationGuideLink = "https://kyma-project.io/#/telemetry-manager/docs/user/integrate-otlp-backend/migration-to-otlp-logs.html"
)

type validator struct {
}

var _ admission.Validator[*telemetryv1beta1.LogPipeline] = &validator{}

func (v *validator) ValidateCreate(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline) (admission.Warnings, error) {
	return validate(ctx, pipeline)
}

func (v *validator) ValidateUpdate(ctx context.Context, _, newPipeline *telemetryv1beta1.LogPipeline) (admission.Warnings, error) {
	return validate(ctx, newPipeline)
}

func (v *validator) ValidateDelete(_ context.Context, _ *telemetryv1beta1.LogPipeline) (admission.Warnings, error) {
	return nil, nil
}

func validate(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline) (admission.Warnings, error) {
	var warnings admission.Warnings

	if err := validateFilterTransform(ctx, pipeline.Spec.Filters, pipeline.Spec.Transforms); err != nil {
		return nil, err
	}

	if logpipelineutils.IsCustomFilterDefined(pipeline.Spec.FluentBitFilters) {
		warnings = append(warnings, renderDeprecationWarning(pipeline.Name, "filters"))
	}

	if logpipelineutils.IsCustomOutputDefined(&pipeline.Spec.Output) {
		warnings = append(warnings, renderDeprecationWarning(pipeline.Name, "output.custom"))
	}

	if logpipelineutils.IsHTTPOutputDefined(&pipeline.Spec.Output) {
		warnings = append(warnings, renderDeprecationWarning(pipeline.Name, "output.http"))
	}

	if logpipelineutils.IsVariablesDefined(pipeline.Spec.FluentBitVariables) {
		warnings = append(warnings, renderDeprecationWarning(pipeline.Name, "variables"))
	}

	if logpipelineutils.IsFilesDefined(pipeline.Spec.FluentBitFiles) {
		warnings = append(warnings, renderDeprecationWarning(pipeline.Name, "files"))
	}

	if logpipelineutils.IsRuntimeInputEnabled(&pipeline.Spec.Input) && pipeline.Spec.Input.Runtime.FluentBitDropLabels != nil {
		warnings = append(warnings, renderDeprecationWarning(pipeline.Name, "input.runtime.dropLabels"))
	}

	if logpipelineutils.IsRuntimeInputEnabled(&pipeline.Spec.Input) && pipeline.Spec.Input.Runtime.FluentBitKeepAnnotations != nil {
		warnings = append(warnings, renderDeprecationWarning(pipeline.Name, "input.runtime.keepAnnotations"))
	}

	return warnings, nil
}

func renderDeprecationWarning(pipelineName string, attribute string) string {
	return fmt.Sprintf("LogPipeline '%s' uses the attribute '%s' which is based on the deprecated FluentBit technology stack. Please migrate to an Open Telemetry based logPipeline instead. See the documentation: %s", pipelineName, attribute, migrationGuideLink)
}

func validateFilterTransform(ctx context.Context, filterSpec []telemetryv1beta1.FilterSpec, transformSpec []telemetryv1beta1.TransformSpec) error {
	err := webhookutils.ValidateFilterTransform(ctx, ottl.SignalTypeLog, filterSpec, transformSpec)
	if err != nil {
		return fmt.Errorf(conditions.MessageForOtelLogPipeline(conditions.ReasonOTTLSpecInvalid), err.Error())
	}

	return nil
}
