package logpipeline

import (
	"context"
	"errors"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
	"github.com/kyma-project/telemetry-manager/internal/tlscert"
)

var errUnsupportedLokiOutput = errors.New("the grafana-loki output is not supported anymore. For integration with a custom Loki installation, use the `custom` output and follow https://kyma-project.io/#/telemetry-manager/user/integration/loki/README")

type TLSCertValidator interface {
	Validate(ctx context.Context, config tlscert.TLSBundle) error
}

type SecretRefValidator interface {
	Validate(ctx context.Context, getter secretref.Getter) error
}

type Validator struct {
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
