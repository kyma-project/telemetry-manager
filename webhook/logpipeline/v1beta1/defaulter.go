package v1beta1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

// +kubebuilder:object:generate=false
var _ webhook.CustomDefaulter = &defaulter{}

type defaulter struct {
	ExcludeNamespaces            []string
	RuntimeInputEnabled          bool
	RuntimeInputKeepOriginalBody bool
	DefaultOTLPOutputProtocol    telemetryv1beta1.OTLPProtocol
	OTLPInputEnabled             bool
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
	if pipeline.Spec.Input.Runtime == nil {
		pipeline.Spec.Input.Runtime = &telemetryv1beta1.LogPipelineRuntimeInput{
			Enabled:          &ld.RuntimeInputEnabled,
			KeepOriginalBody: &ld.RuntimeInputKeepOriginalBody,
		}
	}

	if pipeline.Spec.Input.Runtime.Enabled == nil {
		pipeline.Spec.Input.Runtime.Enabled = &ld.RuntimeInputEnabled
	}

	if *pipeline.Spec.Input.Runtime.Enabled && pipeline.Spec.Input.Runtime.KeepOriginalBody == nil {
		pipeline.Spec.Input.Runtime.KeepOriginalBody = &ld.RuntimeInputKeepOriginalBody
	}

	if *pipeline.Spec.Input.Runtime.Enabled && pipeline.Spec.Input.Runtime.Namespaces == nil {
		pipeline.Spec.Input.Runtime.Namespaces = &telemetryv1beta1.NamespaceSelector{
			Exclude: ld.ExcludeNamespaces,
		}
	}

	if pipeline.Spec.Output.OTLP != nil && pipeline.Spec.Output.OTLP.Protocol == "" {
		pipeline.Spec.Output.OTLP.Protocol = ld.DefaultOTLPOutputProtocol
	}

	if isOTLPPipeline(pipeline) {
		if pipeline.Spec.Input.OTLP == nil {
			pipeline.Spec.Input.OTLP = &telemetryv1beta1.OTLPInput{}
		}

		if pipeline.Spec.Input.OTLP.Enabled == nil {
			pipeline.Spec.Input.OTLP.Enabled = &ld.OTLPInputEnabled
		}

		if ptr.Deref(pipeline.Spec.Input.OTLP.Enabled, false) && pipeline.Spec.Input.OTLP.Namespaces == nil {
			pipeline.Spec.Input.OTLP.Namespaces = &telemetryv1beta1.NamespaceSelector{}
		}
	}
}

func isOTLPPipeline(pipeline *telemetryv1beta1.LogPipeline) bool {
	return pipeline.Spec.Output.OTLP != nil
}
