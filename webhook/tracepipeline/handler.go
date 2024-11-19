package tracepipeline

import (
	"context"
	"encoding/json"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type TracePipelineDefaults struct {
	DefaultOTLPOutputProtocol string
}

type DefaultingWebhookHandler struct {
	defaults TracePipelineDefaults
	decoder  admission.Decoder
}

func NewDefaultingWebhookHandler(scheme *runtime.Scheme) *DefaultingWebhookHandler {
	return &DefaultingWebhookHandler{
		defaults: TracePipelineDefaults{
			DefaultOTLPOutputProtocol: telemetryv1alpha1.OTLPProtocolGRPC,
		},
		decoder: admission.NewDecoder(scheme),
	}
}

func (dh DefaultingWebhookHandler) Handle(ctx context.Context, request admission.Request) admission.Response {
	pipeline := &telemetryv1alpha1.TracePipeline{}

	err := dh.decoder.Decode(request, pipeline)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	dh.applyDefaults(pipeline)

	marshaledPipeline, err := json.Marshal(pipeline)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(request.Object.Raw, marshaledPipeline)
}

func (dh DefaultingWebhookHandler) applyDefaults(pipeline *telemetryv1alpha1.TracePipeline) {
	if pipeline.Spec.Output.OTLP.Protocol == "" {
		pipeline.Spec.Output.OTLP.Protocol = dh.defaults.DefaultOTLPOutputProtocol
	}
}
