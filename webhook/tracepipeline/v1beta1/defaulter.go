package v1beta1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

// +kubebuilder:object:generate=false
var _ webhook.CustomDefaulter = &defaulter{}

type defaulter struct {
	DefaultOTLPOutputProtocol telemetryv1beta1.OTLPProtocol
}

func (td defaulter) Default(ctx context.Context, obj runtime.Object) error {
	pipeline, ok := obj.(*telemetryv1beta1.TracePipeline)
	if !ok {
		return fmt.Errorf("expected an TracePipeline object but got %T", obj)
	}

	td.applyDefaults(pipeline)

	return nil
}

func (td defaulter) applyDefaults(pipeline *telemetryv1beta1.TracePipeline) {
	if pipeline.Spec.Output.OTLP != nil && pipeline.Spec.Output.OTLP.Protocol == "" {
		pipeline.Spec.Output.OTLP.Protocol = td.DefaultOTLPOutputProtocol
	}
}
