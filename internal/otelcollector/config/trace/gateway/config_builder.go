package gateway

import (
	"context"
	"fmt"
	"maps"
	"sort"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
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
	cfg := &Config{
		Base: config.Base{
			Service:    config.DefaultService(make(config.Pipelines)),
			Extensions: config.DefaultExtensions(),
		},
		Receivers:  makeReceiversConfig(),
		Processors: makeProcessorsConfig(opts),
		Exporters:  make(Exporters),
	}

	envVars := make(otlpexporter.EnvVars)

	queueSize := maxQueueSize / len(pipelines)

	for i := range pipelines {
		pipeline := pipelines[i]
		if pipeline.DeletionTimestamp != nil {
			continue
		}

		otlpExporterBuilder := otlpexporter.NewConfigBuilder(
			b.Reader,
			pipeline.Spec.Output.OTLP,
			pipeline.Name,
			queueSize,
			otlpexporter.SignalTypeTrace,
		)
		if err := addComponentsForTracePipeline(ctx, otlpExporterBuilder, &pipeline, cfg, envVars); err != nil {
			return nil, nil, err
		}
	}

	return cfg, envVars, nil
}

func makeReceiversConfig() Receivers {
	return Receivers{
		OTLP: config.OTLPReceiver{
			Protocols: config.ReceiverProtocols{
				HTTP: config.Endpoint{
					Endpoint: fmt.Sprintf("${%s}:%d", config.EnvVarCurrentPodIP, ports.OTLPHTTP),
				},
				GRPC: config.Endpoint{
					Endpoint: fmt.Sprintf("${%s}:%d", config.EnvVarCurrentPodIP, ports.OTLPGRPC),
				},
			},
		},
	}
}

// addComponentsForTracePipeline enriches a Config (exporters, processors, etc.) with components for a given telemetryv1alpha1.TracePipeline.
func addComponentsForTracePipeline(ctx context.Context, otlpExporterBuilder *otlpexporter.ConfigBuilder, pipeline *telemetryv1alpha1.TracePipeline, cfg *Config, envVars otlpexporter.EnvVars) error {
	otlpExporterConfig, otlpExporterEnvVars, err := otlpExporterBuilder.MakeConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to make otlp exporter config: %w", err)
	}

	maps.Copy(envVars, otlpExporterEnvVars)

	otlpExporterID := otlpexporter.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
	cfg.Exporters[otlpExporterID] = Exporter{OTLP: otlpExporterConfig}

	pipelineID := fmt.Sprintf("traces/%s", pipeline.Name)
	cfg.Service.Pipelines[pipelineID] = makePipelineConfig(otlpExporterID)

	return nil
}

func makePipelineConfig(exporterIDs ...string) config.Pipeline {
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
