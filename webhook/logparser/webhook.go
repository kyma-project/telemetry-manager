/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package logparser

import (
	"context"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	logpipelinewebhook "github.com/kyma-project/telemetry-manager/webhook/logpipeline"
)

//go:generate mockery --name DryRunner --filename dryrun.go
type DryRunner interface {
	RunParser(ctx context.Context, parser *telemetryv1alpha1.LogParser) error
}

// +kubebuilder:webhook:path=/validate-logparser,mutating=false,failurePolicy=fail,sideEffects=None,groups=telemetry.kyma-project.io,resources=logparsers,verbs=create;update,versions=v1alpha1,name=vlogparser.kb.io,admissionReviewVersions=v1
type ValidatingWebhookHandler struct {
	client.Client
	dryRunner DryRunner
	decoder   *admission.Decoder
}

func NewValidatingWebhookHandler(client client.Client, dryRunner DryRunner, decoder *admission.Decoder) *ValidatingWebhookHandler {
	return &ValidatingWebhookHandler{
		Client:    client,
		dryRunner: dryRunner,
		decoder:   decoder,
	}
}

func (v *ValidatingWebhookHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := logf.FromContext(ctx)

	logParser := &telemetryv1alpha1.LogParser{}
	if err := v.decoder.Decode(req, logParser); err != nil {
		log.Error(err, "Failed to decode LogParser")
		return admission.Errored(http.StatusBadRequest, err)
	}

	if err := v.validateLogParser(ctx, logParser); err != nil {
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

func (v *ValidatingWebhookHandler) validateLogParser(ctx context.Context, logParser *telemetryv1alpha1.LogParser) error {
	log := logf.FromContext(ctx)
	err := logParser.Validate()
	if err != nil {
		return err
	}

	if err = v.dryRunner.RunParser(ctx, logParser); err != nil {
		log.Error(err, "Failed to validate Fluent Bit parser config")
		return err
	}

	return nil
}

func (v *ValidatingWebhookHandler) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
