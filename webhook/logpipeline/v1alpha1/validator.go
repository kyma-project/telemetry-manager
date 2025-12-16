package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
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
	logPipeline, ok := obj.(*telemetryv1alpha1.LogPipeline)

	if !ok {
		return nil, fmt.Errorf("expected a LogPipeline but got %T", obj)
	}

	return v.validateLogPipeline(ctx, logPipeline)
}

func (v *LogPipelineValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	logPipeline, ok := newObj.(*telemetryv1alpha1.LogPipeline)

	if !ok {
		return nil, fmt.Errorf("expected a LogPipeline but got %T", newObj)
	}

	return v.validateLogPipeline(ctx, logPipeline)
}

func (v *LogPipelineValidator) validateLogPipeline(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) (admission.Warnings, error) {
	var warnings admission.Warnings

	filterSpec, transformSpec := webhookutils.ConvertFilterTransformToBeta(pipeline.Spec.Filters, pipeline.Spec.Transforms)

	if err := validateFilterTransform(ctx, filterSpec, transformSpec); err != nil {
		return nil, err
	}

	if logpipelineutils.IsCustomFilterDefined(pipeline.Spec.FluentBitFilters) {
		warnings = append(warnings, renderDeprecationWarning(pipeline.Name, "filters"))
	}

	if logpipelineutils.IsCustomOutputDefined(&pipeline.Spec.Output) {
		warnings = append(warnings, renderDeprecationWarning(pipeline.Name, "output.custom"))
	}

	if logpipelineutils.IsHTTPDefined(&pipeline.Spec.Output) {
		warnings = append(warnings, renderDeprecationWarning(pipeline.Name, "output.http"))
	}

	if logpipelineutils.IsVariablesDefined(pipeline.Spec.Variables) {
		warnings = append(warnings, renderDeprecationWarning(pipeline.Name, "variables"))
	}

	if logpipelineutils.IsFilesDefined(pipeline.Spec.Files) {
		warnings = append(warnings, renderDeprecationWarning(pipeline.Name, "files"))
	}

	if logpipelineutils.IsApplicationInputEnabled(&pipeline.Spec.Input) && pipeline.Spec.Input.Application.DropLabels != nil {
		warnings = append(warnings, renderDeprecationWarning(pipeline.Name, "input.application.dropLabels"))
	}
	if logpipelineutils.IsApplicationInputEnabled(&pipeline.Spec.Input) && pipeline.Spec.Input.Application.KeepAnnotations != nil {
		warnings = append(warnings, renderDeprecationWarning(pipeline.Name, "input.application.keepAnnotations"))
	}

	return warnings, nil
}

func renderDeprecationWarning(pipelineName string, attribute string) string {
	return fmt.Sprintf("LogPipeline '%s' uses the attribute '%s' which is based on the deprecated FluentBit technology stack. Please migrate to an Open Telemetry based pipeline instead. See the documentation: %s", pipelineName, attribute, migrationGuideLink)
}

func (v *LogPipelineValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validateFilterTransform(ctx context.Context, filterSpec []telemetryv1beta1.FilterSpec, transformSpec []telemetryv1beta1.TransformSpec) error {
	err := webhookutils.ValidateFilterTransform(ctx, ottl.SignalTypeLog, filterSpec, transformSpec)
	if err != nil {
		return fmt.Errorf(conditions.MessageForOtelLogPipeline(conditions.ReasonOTTLSpecInvalid), err.Error())
	}

	return nil
}
