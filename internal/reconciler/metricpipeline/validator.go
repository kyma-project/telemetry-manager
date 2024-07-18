package metricpipeline

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
	"github.com/kyma-project/telemetry-manager/internal/tlscert"
)

type TLSCertValidator interface {
	Validate(ctx context.Context, config tlscert.TLSBundle) error
}

type SecretRefValidator interface {
	Validate(ctx context.Context, getter secretref.Getter) error
}

type validator struct {
	client             client.Reader
	tlsCertValidator   TLSCertValidator
	secretRefValidator SecretRefValidator
	pipelineLock       PipelineLock
}

func NewValidator(client client.Client, pipelineLock PipelineLock) *validator {
	return &validator{
		client:             client,
		tlsCertValidator:   tlscert.New(client),
		secretRefValidator: &secretref.Validator{Client: client},
		pipelineLock:       pipelineLock,
	}
}

func (v *validator) validate(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline) error {
	if err := v.secretRefValidator.Validate(ctx, pipeline); err != nil {
		return err
	}

	if tlsValidationRequired(pipeline) {
		tlsConfig := tlscert.TLSBundle{
			Cert: pipeline.Spec.Output.Otlp.TLS.Cert,
			Key:  pipeline.Spec.Output.Otlp.TLS.Key,
			CA:   pipeline.Spec.Output.Otlp.TLS.CA,
		}

		if err := v.tlsCertValidator.Validate(ctx, tlsConfig); err != nil {
			return err
		}
	}

	if err := v.pipelineLock.IsLockHolder(ctx, pipeline); err != nil {
		return err
	}

	return nil
}
