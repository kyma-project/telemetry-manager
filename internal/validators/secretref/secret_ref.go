package secretref

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
)

type Validator struct {
	Client client.Reader
}

// ValidateTracePipeline validates the secret references in a TracePipeline, ensuring that the references are valid,
// and the referenced Secrets exist and contain the required keys. It returns an error otherwise.
func (v *Validator) ValidateTracePipeline(ctx context.Context, pipeline *telemetryv1beta1.TracePipeline) error {
	return v.validate(ctx, secretref.GetTracePipelineRefs(pipeline))
}

// ValidateMetricPipeline validates the secret references in a MetricPipeline, ensuring that the references are valid,
// and the referenced Secrets exist and contain the required keys. It returns an error otherwise.
func (v *Validator) ValidateMetricPipeline(ctx context.Context, pipeline *telemetryv1beta1.MetricPipeline) error {
	return v.validate(ctx, secretref.GetMetricPipelineRefs(pipeline))
}

// ValidateLogPipeline validates the secret references in a LogPipeline, ensuring that the references are valid,
// and the referenced Secrets exist and contain the required keys. It returns an error otherwise.
func (v *Validator) ValidateLogPipeline(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline) error {
	if pipeline.Spec.Output.OTLP != nil {
		return v.validate(ctx, secretref.GetOTLPOutputRefs(pipeline.Spec.Output.OTLP))
	}

	return v.validate(ctx, secretref.GetLogPipelineRefs(pipeline))
}

func (v *Validator) validate(ctx context.Context, refs []telemetryv1beta1.SecretKeyRef) error {
	for _, ref := range refs {
		if _, err := secretref.GetValue(ctx, v.Client, ref); err != nil {
			return err
		}
	}

	return nil
}
