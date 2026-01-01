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

type TracePipelineValidator struct {
}

var _ webhook.CustomValidator = &TracePipelineValidator{}

func (v *TracePipelineValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return validate(ctx, obj)
}

func (v *TracePipelineValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return validate(ctx, newObj)
}

func (v *TracePipelineValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validateFilterTransform(ctx context.Context, filterSpec []telemetryv1beta1.FilterSpec, transformSpec []telemetryv1beta1.TransformSpec) error {
	err := webhookutils.ValidateFilterTransform(ctx, ottl.SignalTypeTrace, filterSpec, transformSpec)
	if err != nil {
		return fmt.Errorf(conditions.MessageForTracePipeline(conditions.ReasonOTTLSpecInvalid), err.Error())
	}

	return nil
}

func validate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	tracePipeline, ok := obj.(*telemetryv1beta1.TracePipeline)

	if !ok {
		return nil, fmt.Errorf("expected a TracePipeline but got %T", obj)
	}

	return nil, validateFilterTransform(ctx, tracePipeline.Spec.Filters, tracePipeline.Spec.Transforms)
}
