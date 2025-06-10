package gateway

import (
	"context"
	"fmt"
	"maps"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/processors"
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
	CloudProvider string
	Enrichments   processors.Enrichments
	ModuleVersion string
}

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.LogPipeline, opts BuildOptions) (*Config, otlpexporter.EnvVars, error) {
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
			otlpexporter.SignalTypeLog,
		)
		if err := addComponentsForLogPipeline(ctx, otlpExporterBuilder, &pipeline, cfg, envVars); err != nil {
			return nil, nil, err
		}

		pipelineID := fmt.Sprintf("logs/%s", pipeline.Name)
		cfg.Service.Pipelines[pipelineID] = makePipelineServiceConfig(&pipeline)
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

// addComponentsForLogPipeline enriches a Config (exporters, processors, etc.) with components for a given telemetryv1alpha1.LogPipeline.
func addComponentsForLogPipeline(ctx context.Context, otlpExporterBuilder *otlpexporter.ConfigBuilder, pipeline *telemetryv1alpha1.LogPipeline, cfg *Config, envVars otlpexporter.EnvVars) error {
	addNamespaceFilter(pipeline, cfg)
	addInputSourceFilters(pipeline, cfg)

	return addOTLPExporter(ctx, otlpExporterBuilder, pipeline, cfg, envVars)
}

func addNamespaceFilter(pipeline *telemetryv1alpha1.LogPipeline, cfg *Config) {
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
		cfg.Processors.NamespaceFilters[processorID] = makeNamespaceFilterConfig(otlpInput.Namespaces)
	}
}

func addOTLPExporter(ctx context.Context, otlpExporterBuilder *otlpexporter.ConfigBuilder, pipeline *telemetryv1alpha1.LogPipeline, cfg *Config, envVars otlpexporter.EnvVars) error {
	otlpExporterConfig, otlpExporterEnvVars, err := otlpExporterBuilder.MakeConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to make otlp exporter config: %w", err)
	}

	maps.Copy(envVars, otlpExporterEnvVars)

	otlpExporterID := formatOTLPExporterID(pipeline)
	cfg.Exporters[otlpExporterID] = Exporter{OTLP: otlpExporterConfig}

	return nil
}

func addInputSourceFilters(pipeline *telemetryv1alpha1.LogPipeline, cfg *Config) {
	input := pipeline.Spec.Input
	if !logpipelineutils.IsOTLPInputEnabled(input) {
		cfg.Processors.DropIfInputSourceOTLP = makeDropIfInputSourceOTLPConfig()
	}
}
