package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

// +kubebuilder:object:generate=false
var _ webhook.CustomDefaulter = &defaulter{}

type defaulter struct {
	ApplicationInputEnabled          bool
	ApplicationInputKeepOriginalBody bool
}

func (ld defaulter) Default(ctx context.Context, obj runtime.Object) error {
	pipeline, ok := obj.(*telemetryv1alpha1.LogPipeline)
	if !ok {
		return fmt.Errorf("expected an LogPipeline object but got %T", obj)
	}

	ld.applyDefaults(pipeline)

	return nil
}

func (ld defaulter) applyDefaults(pipeline *telemetryv1alpha1.LogPipeline) {
	if pipeline.Spec.Input.Application != nil {
		if pipeline.Spec.Input.Application.Enabled == nil {
			pipeline.Spec.Input.Application.Enabled = &ld.ApplicationInputEnabled
		}

		if pipeline.Spec.Input.Application.KeepOriginalBody == nil {
			pipeline.Spec.Input.Application.KeepOriginalBody = &ld.ApplicationInputKeepOriginalBody
		}
	}
}
