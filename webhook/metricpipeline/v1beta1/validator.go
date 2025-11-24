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

type MetricPipelineValidator struct {
}

var _ webhook.CustomValidator = &MetricPipelineValidator{}

func (v *MetricPipelineValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	metricPipeline, ok := obj.(*telemetryv1beta1.MetricPipeline)

	if !ok {
		return nil, fmt.Errorf("expected a MetricPipeline but got %T", obj)
	}

	return nil, webhookutils.ValidateFilterTransform(ottl.SignalTypeMetric, metricPipeline.Spec.Filters, metricPipeline.Spec.Transforms)
}

func (v *MetricPipelineValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	metricPipeline, ok := newObj.(*telemetryv1beta1.MetricPipeline)

	if !ok {
		return nil, fmt.Errorf("expected a MetricPipeline but got %T", newObj)
	}

	return nil, webhookutils.ValidateFilterTransform(ottl.SignalTypeMetric, metricPipeline.Spec.Filters, metricPipeline.Spec.Transforms)
}

func (v *MetricPipelineValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
