package v1alpha1

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/pipelines"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
	webhookutils "github.com/kyma-project/telemetry-manager/webhook/utils"
)

type validator struct {
	secretAccessValidator *secretref.AccessValidator
}

func newValidator(c client.Client) *validator {
	return &validator{secretAccessValidator: &secretref.AccessValidator{Client: c}}
}

var _ admission.Validator[*telemetryv1alpha1.TracePipeline] = &validator{}

func (v *validator) ValidateCreate(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline) (admission.Warnings, error) {
	return v.validate(ctx, pipeline)
}

func (v *validator) ValidateUpdate(ctx context.Context, _, newPipeline *telemetryv1alpha1.TracePipeline) (admission.Warnings, error) {
	return v.validate(ctx, newPipeline)
}

func (v *validator) ValidateDelete(_ context.Context, _ *telemetryv1alpha1.TracePipeline) (admission.Warnings, error) {
	return nil, nil
}

func (v *validator) validate(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline) (admission.Warnings, error) {
	filterSpec, transformSpec, err := webhookutils.ConvertFilterTransformToBeta(pipeline.Spec.Filters, pipeline.Spec.Transforms)
	if err != nil {
		return nil, err
	}

	if err := validateFilterTransform(ctx, filterSpec, transformSpec); err != nil {
		return nil, err
	}

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get admission request from context: %w", err)
	}

	refs := getSecretRefs(pipeline)
	if err := v.secretAccessValidator.ValidateSecretAccess(ctx, req.UserInfo, refs); err != nil {
		return nil, err
	}

	return nil, nil
}

// getSecretRefs extracts all SecretKeyRefs from a v1alpha1 TracePipeline.
// v1alpha1 is the hub type so there is no ConvertTo method; refs are gathered directly.
func getSecretRefs(pipeline *telemetryv1alpha1.TracePipeline) []telemetryv1beta1.SecretKeyRef {
	if pipeline.Spec.Output.OTLP == nil {
		return nil
	}

	return getRefsFromOTLPOutput(pipeline.Spec.Output.OTLP)
}

func getRefsFromOTLPOutput(out *telemetryv1alpha1.OTLPOutput) []telemetryv1beta1.SecretKeyRef {
	var refs []telemetryv1beta1.SecretKeyRef

	refs = appendRef(refs, &out.Endpoint)

	if out.Authentication != nil && out.Authentication.Basic != nil {
		refs = appendRef(refs, &out.Authentication.Basic.User)
		refs = appendRef(refs, &out.Authentication.Basic.Password)
	}

	if out.Authentication != nil && out.Authentication.OAuth2 != nil {
		refs = appendRef(refs, &out.Authentication.OAuth2.TokenURL)
		refs = appendRef(refs, &out.Authentication.OAuth2.ClientID)
		refs = appendRef(refs, &out.Authentication.OAuth2.ClientSecret)
	}

	for _, h := range out.Headers {
		refs = appendRef(refs, &h.ValueType)
	}

	if out.TLS != nil && !out.TLS.Insecure {
		refs = appendRef(refs, out.TLS.CA)
		refs = appendRef(refs, out.TLS.Cert)
		refs = appendRef(refs, out.TLS.Key)
	}

	return refs
}

func appendRef(refs []telemetryv1beta1.SecretKeyRef, vt *telemetryv1alpha1.ValueType) []telemetryv1beta1.SecretKeyRef {
	if vt != nil && vt.Value == "" && vt.ValueFrom != nil && vt.ValueFrom.SecretKeyRef != nil {
		r := vt.ValueFrom.SecretKeyRef
		refs = append(refs, telemetryv1beta1.SecretKeyRef{
			Name:      r.Name,
			Namespace: r.Namespace,
			Key:       r.Key,
		})
	}

	return refs
}

func validateFilterTransform(ctx context.Context, filterSpec []telemetryv1beta1.FilterSpec, transformSpec []telemetryv1beta1.TransformSpec) error {
	err := webhookutils.ValidateFilterTransform(ctx, pipelines.SignalTypeTrace, filterSpec, transformSpec)
	if err != nil {
		return fmt.Errorf(conditions.MessageForTracePipeline(conditions.ReasonOTTLSpecInvalid), err.Error())
	}

	return nil
}
