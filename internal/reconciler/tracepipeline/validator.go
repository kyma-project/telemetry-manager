package tracepipeline

import (
	"context"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

// Validator validates TracePipeline resources by checking endpoints, TLS certificates, secret references, and pipeline locks.
type Validator struct {
	EndpointValidator      EndpointValidator
	TLSCertValidator       TLSCertValidator
	SecretRefValidator     SecretRefValidator
	PipelineLock           PipelineLock
	TransformSpecValidator TransformSpecValidator
	FilterSpecValidator    FilterSpecValidator
}

// ValidatorOption configures the Validator during initialization.
type ValidatorOption func(*Validator)

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

// WithValidatorPipelineLock sets the pipeline lock for the Validator.
func WithValidatorPipelineLock(lock PipelineLock) ValidatorOption {
	return func(v *Validator) {
		v.PipelineLock = lock
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

func (v *Validator) validate(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline) error {
	if err := v.SecretRefValidator.ValidateTracePipeline(ctx, pipeline); err != nil {
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

func tlsValidationRequired(pipeline *telemetryv1alpha1.TracePipeline) bool {
	otlp := pipeline.Spec.Output.OTLP
	if otlp == nil {
		return false
	}

	if otlp.TLS == nil {
		return false
	}

	return otlp.TLS.Cert != nil || otlp.TLS.Key != nil || otlp.TLS.CA != nil
}
