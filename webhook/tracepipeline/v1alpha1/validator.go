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

	var converted telemetryv1beta1.TracePipeline
	if err := telemetryv1alpha1.Convert_v1alpha1_TracePipeline_To_v1beta1_TracePipeline(pipeline, &converted, nil); err != nil {
		return nil, fmt.Errorf("failed to convert TracePipeline to v1beta1: %w", err)
	}

	refs := secretref.GetSecretRefsTracePipeline(&converted)
	if err := v.secretAccessValidator.ValidateSecretAccess(ctx, req.UserInfo, refs); err != nil {
		return nil, err
	}

	return nil, nil
}

func validateFilterTransform(ctx context.Context, filterSpec []telemetryv1beta1.FilterSpec, transformSpec []telemetryv1beta1.TransformSpec) error {
	err := webhookutils.ValidateFilterTransform(ctx, pipelines.SignalTypeTrace, filterSpec, transformSpec)
	if err != nil {
		return fmt.Errorf(conditions.MessageForTracePipeline(conditions.ReasonOTTLSpecInvalid), err.Error())
	}

	return nil
}
