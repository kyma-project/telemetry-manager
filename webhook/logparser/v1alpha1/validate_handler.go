package v1alpha1

import (
	"context"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type validateHandler struct {
	decoder admission.Decoder
}

func newValidateHandler(scheme *runtime.Scheme) *validateHandler {
	return &validateHandler{
		decoder: admission.NewDecoder(scheme),
	}
}

func (v *validateHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := logf.FromContext(ctx)

	logParser := &telemetryv1alpha1.LogParser{}
	if err := v.decoder.Decode(req, logParser); err != nil {
		log.Error(err, "Failed to decode LogParser")
		return admission.Errored(http.StatusBadRequest, err)
	}

	if err := validateSpec(logParser); err != nil {
		log.Error(err, "LogParser rejected")
		return admission.Errored(http.StatusBadRequest, err)
	}

	return admission.Allowed("LogParser validation successful")
}
