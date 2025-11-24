package v1beta1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
	webhookutils "github.com/kyma-project/telemetry-manager/webhook/utils"
)

type TracePipelineValidator struct {
}

var _ webhook.CustomValidator = &TracePipelineValidator{}

func (v *TracePipelineValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	tracePipeline, ok := obj.(*telemetryv1beta1.TracePipeline)

	if !ok {
		return nil, fmt.Errorf("expected a TracePipeline but got %T", obj)
	}

	return nil, webhookutils.ValidateFilterTransform(ottl.SignalTypeTrace, tracePipeline.Spec.Filters, tracePipeline.Spec.Transforms)
}

func (v *TracePipelineValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	tracePipeline, ok := newObj.(*telemetryv1beta1.TracePipeline)

	if !ok {
		return nil, fmt.Errorf("expected a TracePipeline but got %T", newObj)
	}

	return nil, webhookutils.ValidateFilterTransform(ottl.SignalTypeTrace, tracePipeline.Spec.Filters, tracePipeline.Spec.Transforms)
}

func (v *TracePipelineValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
