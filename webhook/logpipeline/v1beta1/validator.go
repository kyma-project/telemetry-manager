package v1beta1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
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

type LogPipelineValidator struct {
}

var _ webhook.CustomValidator = &LogPipelineValidator{}

func (v *LogPipelineValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return validate(ctx, obj)
}

func (v *LogPipelineValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return validate(ctx, newObj)
}

func (v *LogPipelineValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	var warnings admission.Warnings

	logPipeline, ok := obj.(*telemetryv1beta1.LogPipeline)

	if !ok {
		return nil, fmt.Errorf("expected a LogPipeline but got %T", obj)
	}

	if err := validateFilterTransform(ctx, logPipeline.Spec.Filters, logPipeline.Spec.Transforms); err != nil {
		return nil, err
	}

	if logpipelineutils.IsCustomFilterDefined(logPipeline.Spec.FluentBitFilters) {
		warnings = append(warnings, renderDeprecationWarning(logPipeline.Name, "filters"))
	}

	if logpipelineutils.IsCustomOutputDefined(&logPipeline.Spec.Output) {
		warnings = append(warnings, renderDeprecationWarning(logPipeline.Name, "output.custom"))
	}

	if logpipelineutils.IsHTTPOutputDefined(&logPipeline.Spec.Output) {
		warnings = append(warnings, renderDeprecationWarning(logPipeline.Name, "output.http"))
	}

	if logpipelineutils.IsVariablesDefined(logPipeline.Spec.FluentBitVariables) {
		warnings = append(warnings, renderDeprecationWarning(logPipeline.Name, "variables"))
	}

	if logpipelineutils.IsFilesDefined(logPipeline.Spec.FluentBitFiles) {
		warnings = append(warnings, renderDeprecationWarning(logPipeline.Name, "files"))
	}

	if logpipelineutils.IsRuntimeInputEnabled(&logPipeline.Spec.Input) && logPipeline.Spec.Input.Runtime.FluentBitDropLabels != nil {
		warnings = append(warnings, renderDeprecationWarning(logPipeline.Name, "input.runtime.dropLabels"))
	}

	if logpipelineutils.IsRuntimeInputEnabled(&logPipeline.Spec.Input) && logPipeline.Spec.Input.Runtime.FluentBitKeepAnnotations != nil {
		warnings = append(warnings, renderDeprecationWarning(logPipeline.Name, "input.runtime.keepAnnotations"))
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
