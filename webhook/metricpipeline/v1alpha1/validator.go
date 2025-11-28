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
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
	webhookutils "github.com/kyma-project/telemetry-manager/webhook/utils"
)

type MetricPipelineValidator struct {
}

var _ webhook.CustomValidator = &MetricPipelineValidator{}

func (v *MetricPipelineValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	metricPipeline, ok := obj.(*telemetryv1alpha1.MetricPipeline)

	if !ok {
		return nil, fmt.Errorf("expected a MetricPipeline but got %T", obj)
	}

	filterSpec, transformSpec := webhookutils.ConvertFilterTransformToBeta(metricPipeline.Spec.Filters, metricPipeline.Spec.Transforms)

	return nil, validateFilterTransform(filterSpec, transformSpec)
}

func (v *MetricPipelineValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	metricPipeline, ok := newObj.(*telemetryv1alpha1.MetricPipeline)

	if !ok {
		return nil, fmt.Errorf("expected a MetricPipeline but got %T", newObj)
	}

	filterSpec, transformSpec := webhookutils.ConvertFilterTransformToBeta(metricPipeline.Spec.Filters, metricPipeline.Spec.Transforms)

	return nil, validateFilterTransform(filterSpec, transformSpec)
}

func (v *MetricPipelineValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validateFilterTransform(filterSpec []telemetryv1beta1.FilterSpec, transformSpec []telemetryv1beta1.TransformSpec) error {
	err := webhookutils.ValidateFilterTransform(ottl.SignalTypeMetric, filterSpec, transformSpec)
	if err != nil {
		return fmt.Errorf(conditions.MessageForMetricPipeline(conditions.ReasonOTTLSpecInvalid), err.Error())
	}

	return nil
}
