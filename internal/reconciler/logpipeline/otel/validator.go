package otel

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

type PipelineLock interface {
	TryAcquireLock(ctx context.Context, owner metav1.Object) error
	IsLockHolder(ctx context.Context, owner metav1.Object) error
}

type EndpointValidator interface {
	Validate(ctx context.Context, endpoint *telemetryv1alpha1.ValueType, protocol string) error
}

type TLSCertValidator interface {
	Validate(ctx context.Context, config tlscert.TLSBundle) error
}

type SecretRefValidator interface {
	ValidateLogPipeline(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error
}

type TransformSpecValidator interface {
	Validate(transforms []telemetryv1alpha1.TransformSpec) error
}

type FilterSpecValidator interface {
	Validate(filters []telemetryv1alpha1.FilterSpec) error
}

type Validator struct {
	PipelineLock           PipelineLock
	EndpointValidator      EndpointValidator
	TLSCertValidator       TLSCertValidator
	SecretRefValidator     SecretRefValidator
	TransformSpecValidator TransformSpecValidator
	FilterSpecValidator    FilterSpecValidator
}

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
