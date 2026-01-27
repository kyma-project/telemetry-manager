package v1beta1

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
	webhookutils "github.com/kyma-project/telemetry-manager/webhook/utils"
)

type validator struct {
}

var _ admission.Validator[*telemetryv1beta1.MetricPipeline] = &validator{}

func (v *validator) ValidateCreate(ctx context.Context, pipeline *telemetryv1beta1.MetricPipeline) (admission.Warnings, error) {
	return validate(ctx, pipeline)
}

func (v *validator) ValidateUpdate(ctx context.Context, _, newPipeline *telemetryv1beta1.MetricPipeline) (admission.Warnings, error) {
	return validate(ctx, newPipeline)
}

func (v *validator) ValidateDelete(_ context.Context, _ *telemetryv1beta1.MetricPipeline) (admission.Warnings, error) {
	return nil, nil
}

func validate(ctx context.Context, pipeline *telemetryv1beta1.MetricPipeline) (admission.Warnings, error) {
	return nil, validateFilterTransform(ctx, pipeline.Spec.Filters, pipeline.Spec.Transforms)
}

func validateFilterTransform(ctx context.Context, filterSpec []telemetryv1beta1.FilterSpec, transformSpec []telemetryv1beta1.TransformSpec) error {
	err := webhookutils.ValidateFilterTransform(ctx, ottl.SignalTypeMetric, filterSpec, transformSpec)
	if err != nil {
		return fmt.Errorf(conditions.MessageForMetricPipeline(conditions.ReasonOTTLSpecInvalid), err.Error())
	}

	return nil
}
