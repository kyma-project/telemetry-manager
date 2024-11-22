package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

// +kubebuilder:object:generate=false
var _ webhook.CustomDefaulter = &TracePipelineDefaulter{}

type TracePipelineDefaulter struct {
	DefaultOTLPOutputProtocol string
}

func SetupTracePipelineWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&telemetryv1alpha1.TracePipeline{}).
		WithDefaulter(&TracePipelineDefaulter{
			DefaultOTLPOutputProtocol: telemetryv1alpha1.OTLPProtocolGRPC,
		}).
		Complete()
}

func (td TracePipelineDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	pipeline, ok := obj.(*telemetryv1alpha1.TracePipeline)
	if !ok {
		return fmt.Errorf("expected an TracePipeline object but got %T", obj)
	}

	td.applyDefaults(pipeline)

	return nil
}

func (td TracePipelineDefaulter) applyDefaults(pipeline *telemetryv1alpha1.TracePipeline) {
	if pipeline.Spec.Output.OTLP != nil && pipeline.Spec.Output.OTLP.Protocol == "" {
		pipeline.Spec.Output.OTLP.Protocol = td.DefaultOTLPOutputProtocol
	}
}
