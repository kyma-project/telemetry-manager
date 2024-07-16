package metricpipeline

import (
	"context"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
	"github.com/kyma-project/telemetry-manager/internal/tlscert"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TLSCertValidator interface {
	Validate(ctx context.Context, config tlscert.TLSBundle) error
}

type pipelineValidator struct {
	client           client.Reader
	tlsCertValidator TLSCertValidator
	pipelineLock     PipelineLock
}

func (v *pipelineValidator) validate(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline) error {
	if err := secretref.VerifySecretReference(ctx, v.client, pipeline); err != nil {
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
