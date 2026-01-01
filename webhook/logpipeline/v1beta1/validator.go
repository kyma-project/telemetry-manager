package v1beta1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	sharedtypesutils "github.com/kyma-project/telemetry-manager/internal/utils/sharedtypes"
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

	if isCustomFilterDefined(logPipeline.Spec.FluentBitFilters) {
		warnings = append(warnings, renderDeprecationWarning(logPipeline.Name, "filters"))
	}

	if isCustomOutputDefined(&logPipeline.Spec.Output) {
		warnings = append(warnings, renderDeprecationWarning(logPipeline.Name, "output.custom"))
	}

	if isHTTPDefined(&logPipeline.Spec.Output) {
		warnings = append(warnings, renderDeprecationWarning(logPipeline.Name, "output.http"))
	}

	if isVariablesDefined(logPipeline.Spec.FluentBitVariables) {
		warnings = append(warnings, renderDeprecationWarning(logPipeline.Name, "variables"))
	}

	if isFilesDefined(logPipeline.Spec.FluentBitFiles) {
		warnings = append(warnings, renderDeprecationWarning(logPipeline.Name, "files"))
	}

	if isRuntimeInputEnabled(&logPipeline.Spec.Input) && logPipeline.Spec.Input.Runtime.FluentBitDropLabels != nil {
		warnings = append(warnings, renderDeprecationWarning(logPipeline.Name, "input.runtime.dropLabels"))
	}

	if isRuntimeInputEnabled(&logPipeline.Spec.Input) && logPipeline.Spec.Input.Runtime.FluentBitKeepAnnotations != nil {
		warnings = append(warnings, renderDeprecationWarning(logPipeline.Name, "input.runtime.keepAnnotations"))
	}

	return warnings, nil
}

func renderDeprecationWarning(pipelineName string, attribute string) string {
	return fmt.Sprintf("LogPipeline '%s' uses the attribute '%s' which is based on the deprecated FluentBit technology stack. Please migrate to an Open Telemetry based logPipeline instead. See the documentation: %s", pipelineName, attribute, migrationGuideLink)
}

func isCustomFilterDefined(filters []telemetryv1beta1.FluentBitFilter) bool {
	for _, filter := range filters {
		if filter.Custom != "" {
			return true
		}
	}

	return false
}

func isCustomOutputDefined(o *telemetryv1beta1.LogPipelineOutput) bool {
	return o.FluentBitCustom != ""
}

func isHTTPDefined(o *telemetryv1beta1.LogPipelineOutput) bool {
	return o.FluentBitHTTP != nil && sharedtypesutils.IsValidBeta(&o.FluentBitHTTP.Host)
}

func isVariablesDefined(v []telemetryv1beta1.FluentBitVariable) bool {
	return len(v) > 0
}

func isFilesDefined(v []telemetryv1beta1.FluentBitFile) bool {
	return len(v) > 0
}

func isRuntimeInputEnabled(i *telemetryv1beta1.LogPipelineInput) bool {
	return i.Runtime != nil && ptr.Deref(i.Runtime.Enabled, false)
}

func validateFilterTransform(ctx context.Context, filterSpec []telemetryv1beta1.FilterSpec, transformSpec []telemetryv1beta1.TransformSpec) error {
	err := webhookutils.ValidateFilterTransform(ctx, ottl.SignalTypeLog, filterSpec, transformSpec)
	if err != nil {
		return fmt.Errorf(conditions.MessageForOtelLogPipeline(conditions.ReasonOTTLSpecInvalid), err.Error())
	}

	return nil
}
