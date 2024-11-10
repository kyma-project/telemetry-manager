package logparser

import (
	"context"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	logpipelinewebhook "github.com/kyma-project/telemetry-manager/webhook/logpipeline"
)

type ValidatingWebhookHandler struct {
	decoder admission.Decoder
}

func NewValidatingWebhookHandler(scheme *runtime.Scheme) *ValidatingWebhookHandler {
	return &ValidatingWebhookHandler{
		decoder: admission.NewDecoder(scheme),
	}
}

func (v *ValidatingWebhookHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := logf.FromContext(ctx)

	logParser := &telemetryv1alpha1.LogParser{}
	if err := v.decoder.Decode(req, logParser); err != nil {
		log.Error(err, "Failed to decode LogParser")
		return admission.Errored(http.StatusBadRequest, err)
	}

	if err := validateSpec(logParser); err != nil {
		log.Error(err, "LogParser rejected")

		return admission.Response{
			AdmissionResponse: admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Code:    int32(http.StatusForbidden),
					Reason:  logpipelinewebhook.StatusReasonConfigurationError,
					Message: err.Error(),
				},
			},
		}
	}

	return admission.Allowed("LogParser validation successful")
}
