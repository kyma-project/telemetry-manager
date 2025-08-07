package gateway

import (
	"context"
	"fmt"
	"maps"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
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
	ModuleVersion string
}

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.LogPipeline, opts BuildOptions) (*Config, otlpexporter.EnvVars, error) {
	cfg := newConfig(opts)

	// Iterate over each LogPipeline CR and enrich the config with pipeline-specific components
	queueSize := maxQueueSize / len(pipelines)
	envVars := make(otlpexporter.EnvVars)

	for i := range pipelines {
		pipeline := pipelines[i]

		otlpExporterBuilder := otlpexporter.NewConfigBuilder(
			b.Reader,
			pipeline.Spec.Output.OTLP,
			pipeline.Name,
			queueSize,
			otlpexporter.SignalTypeLog,
		)
		if err := declareComponentsForLogPipeline(ctx, otlpExporterBuilder, &pipeline, cfg, envVars); err != nil {
			return nil, nil, err
		}

		// Assemble the service pipeline for this LogPipeline
		pipelineID := fmt.Sprintf("logs/%s", pipeline.Name)
		cfg.Service.Pipelines[pipelineID] = servicePipelineConfig(&pipeline)
	}

	return cfg, envVars, nil
}

func receiversConfig() Receivers {
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

// declareComponentsForLogPipeline enriches a Config (exporters, processors, etc.) with components for a given telemetryv1alpha1.LogPipeline.
func declareComponentsForLogPipeline(ctx context.Context, otlpExporterBuilder *otlpexporter.ConfigBuilder, pipeline *telemetryv1alpha1.LogPipeline, cfg *Config, envVars otlpexporter.EnvVars) error {
	declareNamespaceFilter(pipeline, cfg)
	declareInputSourceFilters(pipeline, cfg)

	return declareOTLPExporter(ctx, otlpExporterBuilder, pipeline, cfg, envVars)
}

func declareNamespaceFilter(pipeline *telemetryv1alpha1.LogPipeline, cfg *Config) {
	otlpInput := pipeline.Spec.Input.OTLP
	if otlpInput == nil || otlpInput.Disabled {
		// no namespace filter needed
		return
	}

	if cfg.Processors.NamespaceFilters == nil {
		cfg.Processors.NamespaceFilters = make(NamespaceFilters)
	}

	if shouldFilterByNamespace(otlpInput.Namespaces) {
		processorID := formatNamespaceFilterID(pipeline.Name)
		cfg.Processors.NamespaceFilters[processorID] = namespaceFilterProcessorConfig(otlpInput.Namespaces)
	}
}

func declareInputSourceFilters(pipeline *telemetryv1alpha1.LogPipeline, cfg *Config) {
	input := pipeline.Spec.Input
	if !logpipelineutils.IsOTLPInputEnabled(input) {
		cfg.Processors.DropIfInputSourceOTLP = dropIfInputSourceOTLPProcessorConfig()
	}
}

func declareOTLPExporter(ctx context.Context, otlpExporterBuilder *otlpexporter.ConfigBuilder, pipeline *telemetryv1alpha1.LogPipeline, cfg *Config, envVars otlpexporter.EnvVars) error {
	otlpExporterConfig, otlpExporterEnvVars, err := otlpExporterBuilder.MakeConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to make otlp exporter config: %w", err)
	}

	maps.Copy(envVars, otlpExporterEnvVars)

	otlpExporterID := formatOTLPExporterID(pipeline)
	cfg.Exporters[otlpExporterID] = Exporter{OTLP: otlpExporterConfig}

	return nil
}
