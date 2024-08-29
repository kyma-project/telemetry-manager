package tracepipeline

import (
	"context"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

type EndpointValidator interface {
	Validate(ctx context.Context, endpoint *telemetryv1alpha1.ValueType) error
}

type SecretRefValidator interface {
	Validate(ctx context.Context, getter secretref.Getter) error
}

type TLSCertValidator interface {
	Validate(ctx context.Context, config tlscert.TLSBundle) error
}

type Validator struct {
	EndpointValidator  EndpointValidator
	TLSCertValidator   TLSCertValidator
	SecretRefValidator SecretRefValidator
	PipelineLock       PipelineLock
}

func (v *Validator) validate(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline) error {
	if err := v.SecretRefValidator.Validate(ctx, pipeline); err != nil {
		return err
	}

	if pipeline.Spec.Output.Otlp != nil {
		if err := v.EndpointValidator.Validate(ctx, &pipeline.Spec.Output.Otlp.Endpoint); err != nil {
			return err
		}
	}

	if tlsValidationRequired(pipeline) {
		tlsConfig := tlscert.TLSBundle{
			Cert: pipeline.Spec.Output.Otlp.TLS.Cert,
			Key:  pipeline.Spec.Output.Otlp.TLS.Key,
			CA:   pipeline.Spec.Output.Otlp.TLS.CA,
		}

		if err := v.TLSCertValidator.Validate(ctx, tlsConfig); err != nil {
			return err
		}
	}

	if err := v.PipelineLock.IsLockHolder(ctx, pipeline); err != nil {
		return err
	}

	return nil
}

func tlsValidationRequired(pipeline *telemetryv1alpha1.TracePipeline) bool {
	otlp := pipeline.Spec.Output.Otlp
	if otlp == nil {
		return false
	}
	if otlp.TLS == nil {
		return false
	}

	return otlp.TLS.Cert != nil || otlp.TLS.Key != nil || otlp.TLS.CA != nil
}
