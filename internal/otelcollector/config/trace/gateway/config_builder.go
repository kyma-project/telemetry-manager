package gateway

import (
	"context"
	"fmt"
	"sort"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
)

const (
	maxQueueSize = 256 // Maximum number of batches kept in memory before dropping
)

type Builder struct {
	Reader client.Reader
}

type BuildOptions struct {
	ClusterName   string
	ClusterUID    string
	CloudProvider string
	Enrichments   *operatorv1alpha1.EnrichmentSpec
}

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.TracePipeline, opts BuildOptions) (*Config, otlpexporter.EnvVars, error) {
	cfg := newConfig(opts)

	// Iterate over each TracePipeline CR and enrich the config with pipeline-specific components
	envVars := make(otlpexporter.EnvVars)
	queueSize := maxQueueSize / len(pipelines)

	for i := range pipelines {
		pipeline := pipelines[i]

		otlpExporterBuilder := otlpexporter.NewConfigBuilder(
			b.Reader,
			pipeline.Spec.Output.OTLP,
			pipeline.Name,
			queueSize,
			otlpexporter.SignalTypeTrace,
		)
		if err := cfg.addTracePipelineComponents(ctx, otlpExporterBuilder, &pipeline, envVars); err != nil {
			return nil, nil, err
		}

		// Assemble the service pipeline for this TracePipeline
		pipelineID := fmt.Sprintf("traces/%s", pipeline.Name)
		cfg.Service.Pipelines[pipelineID] = pipelineConfig(otlpexporter.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name))
	}

	return cfg, envVars, nil
}

func pipelineConfig(exporterIDs ...string) config.Pipeline {
	sort.Strings(exporterIDs)

	return config.Pipeline{
		Receivers: []string{"otlp"},
		Processors: []string{
			"memory_limiter",
			"k8sattributes",
			"istio_noise_filter",
			"resource/insert-cluster-attributes",
			"service_enrichment",
			"resource/drop-kyma-attributes",
			"batch",
		},
		Exporters: exporterIDs,
	}
}
