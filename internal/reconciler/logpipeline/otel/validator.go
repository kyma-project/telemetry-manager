package otel

import (
	"context"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

// Validator validates LogPipeline resources with OTLP output by checking endpoints, TLS certificates, secret references, and pipeline locks.
type Validator struct {
	PipelineLock           PipelineLock
	EndpointValidator      EndpointValidator
	TLSCertValidator       TLSCertValidator
	SecretRefValidator     SecretRefValidator
	TransformSpecValidator TransformSpecValidator
	FilterSpecValidator    FilterSpecValidator
}

// ValidatorOption configures the Validator during initialization.
type ValidatorOption func(*Validator)

// WithValidatorPipelineLock sets the pipeline lock for the Validator.
func WithValidatorPipelineLock(lock PipelineLock) ValidatorOption {
	return func(v *Validator) {
		v.PipelineLock = lock
	}
}

// WithEndpointValidator sets the endpoint validator for the Validator.
func WithEndpointValidator(validator EndpointValidator) ValidatorOption {
	return func(v *Validator) {
		v.EndpointValidator = validator
	}
}

// WithTLSCertValidator sets the TLS certificate validator for the Validator.
func WithTLSCertValidator(validator TLSCertValidator) ValidatorOption {
	return func(v *Validator) {
		v.TLSCertValidator = validator
	}
}

// WithSecretRefValidator sets the secret reference validator for the Validator.
func WithSecretRefValidator(validator SecretRefValidator) ValidatorOption {
	return func(v *Validator) {
		v.SecretRefValidator = validator
	}
}

// WithTransformSpecValidator sets the transform spec validator for the Validator.
func WithTransformSpecValidator(validator TransformSpecValidator) ValidatorOption {
	return func(v *Validator) {
		v.TransformSpecValidator = validator
	}
}

// WithFilterSpecValidator sets the filter spec validator for the Validator.
func WithFilterSpecValidator(validator FilterSpecValidator) ValidatorOption {
	return func(v *Validator) {
		v.FilterSpecValidator = validator
	}
}

// NewValidator creates a new Validator with the provided options.
func NewValidator(opts ...ValidatorOption) *Validator {
	v := &Validator{}

	for _, opt := range opts {
		opt(v)
	}

	return v
}

// Validate validates the LogPipeline resource by checking secret references, endpoint configuration, TLS certificates, and pipeline lock status.
func (v *Validator) Validate(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
	if err := v.SecretRefValidator.ValidateLogPipeline(ctx, pipeline); err != nil {
		return err
	}

	if pipeline.Spec.Output.OTLP != nil {
		if err := v.EndpointValidator.Validate(ctx, &pipeline.Spec.Output.OTLP.Endpoint, pipeline.Spec.Output.OTLP.Protocol); err != nil {
			return err
		}
	}

	if tlsValidationRequired(pipeline) {
		tlsConfig := tlscert.TLSBundle{
			Cert: pipeline.Spec.Output.OTLP.TLS.Cert,
			Key:  pipeline.Spec.Output.OTLP.TLS.Key,
			CA:   pipeline.Spec.Output.OTLP.TLS.CA,
		}

		if err := v.TLSCertValidator.Validate(ctx, tlsConfig); err != nil {
			return err
		}
	}

	if err := v.PipelineLock.IsLockHolder(ctx, pipeline); err != nil {
		return err
	}

	if err := v.TransformSpecValidator.Validate(pipeline.Spec.Transforms); err != nil {
		return err
	}

	if err := v.FilterSpecValidator.Validate(pipeline.Spec.Filters); err != nil {
		return err
	}

	return nil
}

// tlsValidationRequired checks if TLS validation is required for the pipeline.
// Returns true if the pipeline has OTLP output with TLS configuration containing cert, key, or CA.
func tlsValidationRequired(pipeline *telemetryv1alpha1.LogPipeline) bool {
	otlp := pipeline.Spec.Output.OTLP
	if otlp == nil {
		return false
	}

	if otlp.TLS == nil {
		return false
	}

	return otlp.TLS.Cert != nil || otlp.TLS.Key != nil || otlp.TLS.CA != nil
}
