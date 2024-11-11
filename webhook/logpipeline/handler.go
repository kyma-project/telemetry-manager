package logpipeline

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
)

const (
	StatusReasonConfigurationError = "InvalidConfiguration"
)

type ValidatingWebhookHandler struct {
	client  client.Client
	decoder admission.Decoder
}

func NewValidatingWebhookHandler(
	client client.Client,
	scheme *runtime.Scheme,
) *ValidatingWebhookHandler {
	return &ValidatingWebhookHandler{
		client:  client,
		decoder: admission.NewDecoder(scheme),
	}
}

func (h *ValidatingWebhookHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := logf.FromContext(ctx)

	logPipeline := &telemetryv1alpha1.LogPipeline{}
	if err := h.decoder.Decode(req, logPipeline); err != nil {
		log.Error(err, "Failed to decode LogPipeline")
		return admission.Errored(http.StatusBadRequest, err)
	}

	if err := h.validateLogPipeline(ctx, logPipeline); err != nil {
		log.Error(err, "LogPipeline rejected")

		return admission.Response{
			AdmissionResponse: admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Code:    int32(http.StatusForbidden),
					Reason:  StatusReasonConfigurationError,
					Message: err.Error(),
				},
			},
		}
	}

	var warnMsg []string

	if logpipelineutils.ContainsCustomPlugin(logPipeline) {
		helpText := "https://kyma-project.io/#/telemetry-manager/user/02-logs"
		msg := fmt.Sprintf("Logpipeline '%s' uses unsupported custom filters or outputs. We recommend changing the pipeline to use supported filters or output. See the documentation: %s", logPipeline.Name, helpText)
		warnMsg = append(warnMsg, msg)
	}

	if len(warnMsg) != 0 {
		return admission.Response{
			AdmissionResponse: admissionv1.AdmissionResponse{
				Allowed:  true,
				Warnings: warnMsg,
			},
		}
	}

	return admission.Allowed("LogPipeline validation successful")
}

func (h *ValidatingWebhookHandler) validateLogPipeline(ctx context.Context, logPipeline *telemetryv1alpha1.LogPipeline) error {
	log := logf.FromContext(ctx)

	var logPipelines telemetryv1alpha1.LogPipelineList
	if err := h.client.List(ctx, &logPipelines); err != nil {
		return err
	}

	if err := validatePipelineLimit(logPipeline, &logPipelines); err != nil {
		log.Error(err, "Maximum number of log pipelines reached")
		return err
	}

	if err := validateSpec(logPipeline); err != nil {
		log.Error(err, "Log pipeline spec validation failed")
		return err
	}

	if err := validateVariables(logPipeline, &logPipelines); err != nil {
		log.Error(err, "Log pipeline variable validation failed")
		return err
	}

	if err := validateFiles(logPipeline, &logPipelines); err != nil {
		log.Error(err, "Log pipeline file validation failed")
		return err
	}

	return nil
}
