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

	config  *Config
	envVars otlpexporter.EnvVars
}

type BuildOptions struct {
	ClusterName   string
	ClusterUID    string
	CloudProvider string
	Enrichments   *operatorv1alpha1.EnrichmentSpec
	ModuleVersion string
}

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.LogPipeline, opts BuildOptions) (*Config, otlpexporter.EnvVars, error) {
	b.config = newConfig(opts)
	b.envVars = make(otlpexporter.EnvVars)

	// Iterate over each LogPipeline CR and enrich the config with pipeline-specific components
	queueSize := maxQueueSize / len(pipelines)

	for i := range pipelines {
		pipeline := pipelines[i]

		otlpExporterBuilder := otlpexporter.NewConfigBuilder(
			b.Reader,
			pipeline.Spec.Output.OTLP,
			pipeline.Name,
			queueSize,
			otlpexporter.SignalTypeLog,
		)
		if err := b.addComponentsForLogPipeline(ctx, otlpExporterBuilder, &pipeline); err != nil {
			return nil, nil, err
		}

		// Assemble the service pipeline for this LogPipeline
		pipelineID := fmt.Sprintf("logs/%s", pipeline.Name)
		b.config.Service.Pipelines[pipelineID] = servicePipelineConfig(&pipeline)
	}

	return b.config, b.envVars, nil
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

// addComponentsForLogPipeline enriches a Config (exporters, processors, etc.) with components for a given telemetryv1alpha1.LogPipeline.
func (b *Builder) addComponentsForLogPipeline(ctx context.Context, otlpExporterBuilder *otlpexporter.ConfigBuilder, pipeline *telemetryv1alpha1.LogPipeline) error {
	b.addNamespaceFilter(pipeline)
	b.addInputSourceFilters(pipeline)

	return b.addOTLPExporter(ctx, otlpExporterBuilder, pipeline)
}

func (b *Builder) addNamespaceFilter(pipeline *telemetryv1alpha1.LogPipeline) {
	otlpInput := pipeline.Spec.Input.OTLP
	if otlpInput == nil || otlpInput.Disabled {
		// no namespace filter needed
		return
	}

	if b.config.Processors.NamespaceFilters == nil {
		b.config.Processors.NamespaceFilters = make(NamespaceFilters)
	}

	if shouldFilterByNamespace(otlpInput.Namespaces) {
		processorID := formatNamespaceFilterID(pipeline.Name)
		b.config.Processors.NamespaceFilters[processorID] = namespaceFilterProcessorConfig(otlpInput.Namespaces)
	}
}

func (b *Builder) addInputSourceFilters(pipeline *telemetryv1alpha1.LogPipeline) {
	input := pipeline.Spec.Input
	if !logpipelineutils.IsOTLPInputEnabled(input) {
		b.config.Processors.DropIfInputSourceOTLP = dropIfInputSourceOTLPProcessorConfig()
	}
}

func (b *Builder) addOTLPExporter(ctx context.Context, otlpExporterBuilder *otlpexporter.ConfigBuilder, pipeline *telemetryv1alpha1.LogPipeline) error {
	otlpExporterConfig, otlpExporterEnvVars, err := otlpExporterBuilder.MakeConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to make otlp exporter config: %w", err)
	}

	maps.Copy(b.envVars, otlpExporterEnvVars)

	otlpExporterID := formatOTLPExporterID(pipeline)
	b.config.Exporters[otlpExporterID] = Exporter{OTLP: otlpExporterConfig}

	return nil
}
