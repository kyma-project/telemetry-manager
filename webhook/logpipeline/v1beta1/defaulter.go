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
	RuntimeInputEnabled          bool
	RuntimeInputKeepOriginalBody bool
	DefaultOTLPOutputProtocol    telemetryv1beta1.OTLPProtocol
}

func (ld defaulter) Default(ctx context.Context, obj runtime.Object) error {
	pipeline, ok := obj.(*telemetryv1beta1.LogPipeline)
	if !ok {
		return fmt.Errorf("expected an LogPipeline object but got %T", obj)
	}

	ld.applyDefaults(pipeline)

	return nil
}

func (ld defaulter) applyDefaults(pipeline *telemetryv1beta1.LogPipeline) {
	if pipeline.Spec.Input.Runtime != nil {
		if pipeline.Spec.Input.Runtime.Enabled == nil {
			pipeline.Spec.Input.Runtime.Enabled = &ld.RuntimeInputEnabled
		}

		if pipeline.Spec.Input.Runtime.KeepOriginalBody == nil {
			pipeline.Spec.Input.Runtime.KeepOriginalBody = &ld.RuntimeInputKeepOriginalBody
		}
	}

	if pipeline.Spec.Output.OTLP != nil && pipeline.Spec.Output.OTLP.Protocol == "" {
		pipeline.Spec.Output.OTLP.Protocol = ld.DefaultOTLPOutputProtocol
	}
}
