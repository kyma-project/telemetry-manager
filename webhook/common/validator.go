package common

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
	webhookutils "github.com/kyma-project/telemetry-manager/webhook/utils"
)

// getSignalType returns the appropriate signal type for a pipeline type
func getSignalType[T runtime.Object]() ottl.SignalType {
	var zero T
	switch any(zero).(type) {
	case *telemetryv1beta1.MetricPipeline, *telemetryv1alpha1.MetricPipeline:
		return ottl.SignalTypeMetric
	case *telemetryv1beta1.LogPipeline, *telemetryv1alpha1.LogPipeline:
		return ottl.SignalTypeLog
	case *telemetryv1beta1.TracePipeline, *telemetryv1alpha1.TracePipeline:
		return ottl.SignalTypeTrace
	default:
		panic(fmt.Sprintf("unsupported pipeline type for validator: %T", zero))
	}
}

// PipelineValidator is a generic validator for telemetry pipelines
type PipelineValidator[T runtime.Object] struct {
	signalType           ottl.SignalType
	extractFilters       func(T) []telemetryv1beta1.FilterSpec
	extractTransforms    func(T) []telemetryv1beta1.TransformSpec
	customWarningChecker func(T) (admission.Warnings, bool)
}

var _ webhook.CustomValidator = &PipelineValidator[runtime.Object]{}

// ValidatorBuilder provides a fluent API for building pipeline validators
type ValidatorBuilder[T runtime.Object] struct {
	validator *PipelineValidator[T]
}

// NewPipelineValidator creates a new validator builder.
// The signal type is automatically determined from the pipeline type
func NewPipelineValidator[T runtime.Object]() *ValidatorBuilder[T] {
	return &ValidatorBuilder[T]{
		validator: &PipelineValidator[T]{
			signalType: getSignalType[T](),
		},
	}
}

// WithFilterExtractor sets the function to extract filters from the pipeline
func (b *ValidatorBuilder[T]) WithFilterExtractor(fn func(T) []telemetryv1beta1.FilterSpec) *ValidatorBuilder[T] {
	b.validator.extractFilters = fn
	return b
}

// WithTransformExtractor sets the function to extract transforms from the pipeline
func (b *ValidatorBuilder[T]) WithTransformExtractor(fn func(T) []telemetryv1beta1.TransformSpec) *ValidatorBuilder[T] {
	b.validator.extractTransforms = fn
	return b
}

// WithCustomWarningCheck sets a custom warning checker function
func (b *ValidatorBuilder[T]) WithCustomWarningCheck(fn func(T) (admission.Warnings, bool)) *ValidatorBuilder[T] {
	b.validator.customWarningChecker = fn
	return b
}

// Build returns the configured validator
func (b *ValidatorBuilder[T]) Build() *PipelineValidator[T] {
	return b.validator
}

func (v *PipelineValidator[T]) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	typedObj, ok := obj.(T)
	if !ok {
		var zero T
		return nil, fmt.Errorf("expected a %T but got %T", zero, obj)
	}

	// Check for custom warnings if configured
	if v.customWarningChecker != nil {
		if warnings, hasWarnings := v.customWarningChecker(typedObj); hasWarnings {
			return warnings, nil
		}
	}

	// Validate filters and transforms
	if err := v.validateFilterTransform(typedObj); err != nil {
		return nil, err
	}

	return nil, nil
}

func (v *PipelineValidator[T]) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	typedObj, ok := newObj.(T)
	if !ok {
		var zero T
		return nil, fmt.Errorf("expected a %T but got %T", zero, newObj)
	}

	// Check for custom warnings if configured
	if v.customWarningChecker != nil {
		if warnings, hasWarnings := v.customWarningChecker(typedObj); hasWarnings {
			return warnings, nil
		}
	}

	// Validate filters and transforms
	if err := v.validateFilterTransform(typedObj); err != nil {
		return nil, err
	}

	return nil, nil
}

func (v *PipelineValidator[T]) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *PipelineValidator[T]) validateFilterTransform(obj T) error {
	var (
		filters    []telemetryv1beta1.FilterSpec
		transforms []telemetryv1beta1.TransformSpec
	)

	if v.extractFilters != nil {
		filters = v.extractFilters(obj)
	}

	if v.extractTransforms != nil {
		transforms = v.extractTransforms(obj)
	}

	return webhookutils.ValidateFilterTransform(v.signalType, filters, transforms)
}
