package v1alpha1

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

// +kubebuilder:object:generate=false
var _ admission.Defaulter[*telemetryv1alpha1.LogPipeline] = &defaulter{}

type defaulter struct {
	ApplicationInputEnabled          bool
	ApplicationInputKeepOriginalBody bool
	DefaultOTLPProtocol              string
}

func (ld defaulter) Default(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
	if pipeline.Spec.Input.Application == nil {
		pipeline.Spec.Input.Application = &telemetryv1alpha1.LogPipelineApplicationInput{
			Enabled:          &ld.ApplicationInputEnabled,
			KeepOriginalBody: &ld.ApplicationInputKeepOriginalBody,
		}
	}

	if pipeline.Spec.Input.Application.Enabled == nil {
		pipeline.Spec.Input.Application.Enabled = &ld.ApplicationInputEnabled
	}

	if pipeline.Spec.Input.Application.KeepOriginalBody == nil {
		pipeline.Spec.Input.Application.KeepOriginalBody = &ld.ApplicationInputKeepOriginalBody
	}

	if isOTLPPipeline(pipeline) {
		if pipeline.Spec.Input.OTLP == nil {
			pipeline.Spec.Input.OTLP = &telemetryv1alpha1.OTLPInput{}
		}

		if pipeline.Spec.Input.OTLP.Namespaces == nil {
			pipeline.Spec.Input.OTLP.Namespaces = &telemetryv1alpha1.NamespaceSelector{}
		}
	}

	if pipeline.Spec.Output.OTLP != nil && pipeline.Spec.Output.OTLP.Protocol == "" {
		pipeline.Spec.Output.OTLP.Protocol = ld.DefaultOTLPProtocol
	}

	return nil
}

func isOTLPPipeline(pipeline *telemetryv1alpha1.LogPipeline) bool {
	return pipeline.Spec.Output.OTLP != nil
}
