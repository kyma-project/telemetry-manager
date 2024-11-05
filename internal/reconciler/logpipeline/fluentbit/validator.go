package fluentbit

import (
	"context"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/validators/endpoint"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

type EndpointValidator interface {
	Validate(ctx context.Context, endpoint *telemetryv1alpha1.ValueType, protocol string) error
}

type TLSCertValidator interface {
	Validate(ctx context.Context, config tlscert.TLSBundle) error
}

type SecretRefValidator interface {
	ValidateLogPipeline(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error
}

type Validator struct {
	EndpointValidator  EndpointValidator
	TLSCertValidator   TLSCertValidator
	SecretRefValidator SecretRefValidator
}

func (v *Validator) validate(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
	if err := v.SecretRefValidator.ValidateLogPipeline(ctx, pipeline); err != nil {
		return err
	}

	if pipeline.Spec.Output.HTTP != nil {
		if err := v.EndpointValidator.Validate(ctx, &pipeline.Spec.Output.HTTP.Host, endpoint.FluentdProtocolHTTP); err != nil {
			return err
		}
	}

	if tlsValidationRequired(pipeline) {
		tlsConfig := tlscert.TLSBundle{
			Cert: pipeline.Spec.Output.HTTP.TLS.Cert,
			Key:  pipeline.Spec.Output.HTTP.TLS.Key,
			CA:   pipeline.Spec.Output.HTTP.TLS.CA,
		}

		if err := v.TLSCertValidator.Validate(ctx, tlsConfig); err != nil {
			return err
		}
	}

	return nil
}

func tlsValidationRequired(pipeline *telemetryv1alpha1.LogPipeline) bool {
	http := pipeline.Spec.Output.HTTP
	if http == nil {
		return false
	}

	return http.TLS.Cert != nil || http.TLS.Key != nil || http.TLS.CA != nil
}
