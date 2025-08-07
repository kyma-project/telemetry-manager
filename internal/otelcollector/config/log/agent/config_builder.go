package agent

import (
	"context"
	"fmt"
	"maps"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
)

type BuilderConfig struct {
	GatewayOTLPServiceName types.NamespacedName
}
type Builder struct {
	Reader client.Reader
	Config BuilderConfig
}

// Currently the queue is disabled. So set the size to 0
const queueSize = 0

type BuildOptions struct {
	InstrumentationScopeVersion string
	AgentNamespace              string
	ClusterName                 string
	ClusterUID                  string
	CloudProvider               string
	Enrichments                 *operatorv1alpha1.EnrichmentSpec
}

func (b *Builder) Build(ctx context.Context, logPipelines []telemetryv1alpha1.LogPipeline, opts BuildOptions) (*Config, otlpexporter.EnvVars, error) {
	// Fill out the static parts of the config (service, extensions, processors, exporters)
	service := config.DefaultService(make(config.Pipelines))
	service.Extensions = append(service.Extensions, "file_storage")
	cfg := &Config{
		Service:    service,
		Extensions: extensionsConfig(),
		Receivers:  make(Receivers),
		Processors: processorsConfig(opts),
		Exporters:  make(Exporters),
	}

	envVars := make(otlpexporter.EnvVars)

	// Iterate over each LogPipeline CR and enrich the config with pipeline-specific components
	for i := range logPipelines {
		pipeline := logPipelines[i]

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

		pipelineID := fmt.Sprintf("logs/%s", pipeline.Name)
		cfg.Service.Pipelines[pipelineID] = pipelineConfig(fmt.Sprintf("filelog/%s", pipeline.Name), otlpexporter.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name))
	}

	// Return the assembled config and any environment variables needed for exporters
	return cfg, envVars, nil
}

// declareComponentsForLogPipeline enriches a Config (exporters, processors, etc.) with components for a given telemetryv1alpha1.LogPipeline.
func declareComponentsForLogPipeline(ctx context.Context, otlpExporterBuilder *otlpexporter.ConfigBuilder, pipeline *telemetryv1alpha1.LogPipeline, cfg *Config, envVars otlpexporter.EnvVars) error {
	declareFileLogReceiver(pipeline, cfg)
	return declareOTLPExporter(ctx, otlpExporterBuilder, pipeline, cfg, envVars)
}

func declareFileLogReceiver(pipeline *telemetryv1alpha1.LogPipeline, cfg *Config) {
	receiver := fileLogReceiverConfig(*pipeline)

	otlpReceiverID := fmt.Sprintf("filelog/%s", pipeline.Name)
	cfg.Receivers[otlpReceiverID] = Receiver{FileLog: receiver}
}

func declareOTLPExporter(ctx context.Context, otlpExporterBuilder *otlpexporter.ConfigBuilder, pipeline *telemetryv1alpha1.LogPipeline, cfg *Config, envVars otlpexporter.EnvVars) error {
	otlpExporterConfig, otlpExporterEnvVars, err := otlpExporterBuilder.MakeConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to make otlp exporter config: %w", err)
	}

	maps.Copy(envVars, otlpExporterEnvVars)

	otlpExporterID := otlpexporter.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
	cfg.Exporters[otlpExporterID] = Exporter{OTLP: otlpExporterConfig}

	return nil
}

// Each pipeline will have one receiver and one exporter
func pipelineConfig(receiverID, exporterID string) config.Pipeline {
	return config.Pipeline{
		Receivers: []string{receiverID},
		Processors: []string{
			"memory_limiter",
			"transform/set-instrumentation-scope-runtime",
			"k8sattributes",
			"resource/insert-cluster-attributes",
			"service_enrichment",
			"resource/drop-kyma-attributes",
		},
		Exporters: []string{exporterID},
	}
}
