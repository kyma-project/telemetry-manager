package gateway

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
)

const (
	maxQueueSize = 256 // Maximum number of batches kept in memory before dropping
)

type Builder struct {
	Reader client.Reader
}

type BuildOptions struct {
	GatewayNamespace            string
	InstrumentationScopeVersion string
	ClusterName                 string
	ClusterUID                  string
	CloudProvider               string
	Enrichments                 *operatorv1alpha1.EnrichmentSpec
}

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.MetricPipeline, opts BuildOptions) (*Config, otlpexporter.EnvVars, error) {
	cfg := newConfig(opts)

	// Iterate over each MetricPipeline CR and enrich the config with pipeline-specific components
	envVars := make(otlpexporter.EnvVars)
	queueSize := maxQueueSize / len(pipelines)

	for i := range pipelines {
		pipeline := pipelines[i]

		otlpExporterBuilder := otlpexporter.NewConfigBuilder(
			b.Reader,
			pipeline.Spec.Output.OTLP,
			pipeline.Name,
			queueSize,
			otlpexporter.SignalTypeMetric,
		)
		if err := cfg.addMetricPipelineComponents(ctx, otlpExporterBuilder, &pipeline, envVars); err != nil {
			return nil, nil, err
		}

		// Assemble the service pipelines for this MetricPipeline using Config methods
		cfg.addInputPipeline(pipeline.Name)
		cfg.addEnrichmentPipeline(pipeline.Name)
		cfg.addOutputPipeline(&pipeline)
	}

	return cfg, envVars, nil
}

// declare pipeline-specific components using lower-case Config methods

func shouldFilterByNamespace(namespaceSelector *telemetryv1alpha1.NamespaceSelector) bool {
	return namespaceSelector != nil && (len(namespaceSelector.Include) > 0 || len(namespaceSelector.Exclude) > 0)
}
