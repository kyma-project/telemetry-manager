package v1alpha1

import (
	"context"
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-tracepipeline,mutating=false,failurePolicy=fail,sideEffects=None,groups=telemetry.kyma-project.io,resources=tracepipelines,verbs=create;update,versions=v1alpha1,name=validating-tracepipelines.kyma-project.io,admissionReviewVersions=v1;v1beta1

type TracePipelineValidator struct {
}

var _ webhook.CustomValidator = &TracePipelineValidator{}

func (v *TracePipelineValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	metricPipeline, ok := obj.(*telemetryv1alpha1.TracePipeline)

	if !ok {
		return nil, fmt.Errorf("expected a MetricPipeline but got %T", obj)
	}

	return nil, validateFilterTransform(ottl.SignalTypeTrace, metricPipeline.Spec.Filters, metricPipeline.Spec.Transforms)
}

func (v *TracePipelineValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	metricPipeline, ok := newObj.(*telemetryv1alpha1.TracePipeline)

	if !ok {
		return nil, fmt.Errorf("expected a MetricPipeline but got %T", newObj)
	}
	return nil, validateFilterTransform(ottl.SignalTypeTrace, metricPipeline.Spec.Filters, metricPipeline.Spec.Transforms)
}

func (v *TracePipelineValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
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
