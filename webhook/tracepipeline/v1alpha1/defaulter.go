package v1alpha1

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

// +kubebuilder:object:generate=false
var _ admission.Defaulter[*telemetryv1alpha1.TracePipeline] = &defaulter{}

type defaulter struct {
	DefaultOTLPOutputProtocol string
}

func (td defaulter) Default(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline) error {
	if pipeline.Spec.Output.OTLP != nil && pipeline.Spec.Output.OTLP.Protocol == "" {
		pipeline.Spec.Output.OTLP.Protocol = td.DefaultOTLPOutputProtocol
	}

	return nil
}
