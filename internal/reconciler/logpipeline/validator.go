package logpipeline

import (
	"context"
	"errors"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/validators/endpoint"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

var errUnsupportedLokiOutput = errors.New("the grafana-loki output is not supported anymore. For integration with a custom Loki installation, use the `custom` output and follow https://kyma-project.io/#/telemetry-manager/user/integration/loki/README")

type EndpointValidator interface {
	Validate(ctx context.Context, endpoint endpoint.Endpoint) error
}

type TLSCertValidator interface {
	Validate(ctx context.Context, config tlscert.TLSBundle) error
}

type SecretRefValidator interface {
	Validate(ctx context.Context, getter secretref.Getter) error
}

type Validator struct {
	EndpointValidator  EndpointValidator
	TLSCertValidator   TLSCertValidator
	SecretRefValidator SecretRefValidator
}

func (v *Validator) validate(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
	if pipeline.Spec.Output.IsLokiDefined() {
		return errUnsupportedLokiOutput
	}

	if err := v.SecretRefValidator.Validate(ctx, pipeline); err != nil {
		return err
	}

	if pipeline.Spec.Output.HTTP != nil {
		endpoint := endpoint.Endpoint{
			Host: &pipeline.Spec.Output.HTTP.Host,
			Port: pipeline.Spec.Output.HTTP.Port,
		}
		if err := v.EndpointValidator.Validate(ctx, endpoint); err != nil {
			return err
		}
	}

	if tlsValidationRequired(pipeline) {
		tlsConfig := tlscert.TLSBundle{
			Cert: pipeline.Spec.Output.HTTP.TLSConfig.Cert,
			Key:  pipeline.Spec.Output.HTTP.TLSConfig.Key,
			CA:   pipeline.Spec.Output.HTTP.TLSConfig.CA,
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
	return http.TLSConfig.Cert != nil || http.TLSConfig.Key != nil || http.TLSConfig.CA != nil
}
