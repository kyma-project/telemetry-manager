package logpipeline

import (
	"context"
	"encoding/json"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type LogPipelineDefaults struct {
	ApplicationInputEnabled          bool
	ApplicationInputKeepOriginalBody bool
}

type DefaultingWebhookHandler struct {
	defaults LogPipelineDefaults
	decoder  admission.Decoder
}

func NewDefaultingWebhookHandler(scheme *runtime.Scheme) *DefaultingWebhookHandler {
	return &DefaultingWebhookHandler{
		defaults: LogPipelineDefaults{
			ApplicationInputEnabled:          true,
			ApplicationInputKeepOriginalBody: true,
		},
		decoder: admission.NewDecoder(scheme),
	}
}

func (dh DefaultingWebhookHandler) Handle(ctx context.Context, request admission.Request) admission.Response {
	pipeline := &telemetryv1alpha1.LogPipeline{}

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

func (dh DefaultingWebhookHandler) applyDefaults(pipeline *telemetryv1alpha1.LogPipeline) {
	if pipeline.Spec.Input.Application != nil {
		if pipeline.Spec.Input.Application.Enabled == nil {
			pipeline.Spec.Input.Application.Enabled = &dh.defaults.ApplicationInputEnabled
		}

		if pipeline.Spec.Input.Application.KeepOriginalBody == nil {
			pipeline.Spec.Input.Application.KeepOriginalBody = &dh.defaults.ApplicationInputKeepOriginalBody
		}
	}
}
