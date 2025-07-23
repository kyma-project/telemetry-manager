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
	cfg := &Config{
		Service:    config.DefaultService(make(config.Pipelines)),
		Extensions: makeExtensionsConfig(),
		Receivers:  make(Receivers),
		Processors: makeProcessorsConfig(opts),
		Exporters:  make(Exporters),
	}

	envVars := make(otlpexporter.EnvVars)

	for i := range logPipelines {
		pipeline := logPipelines[i]
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
	}

	return cfg, envVars, nil
}

// addComponentsForLogPipeline enriches a Config (exporters, processors, etc.) with components for a given telemetryv1alpha1.LogPipeline.
func addComponentsForLogPipeline(ctx context.Context, otlpExporterBuilder *otlpexporter.ConfigBuilder, pipeline *telemetryv1alpha1.LogPipeline, cfg *Config, envVars otlpexporter.EnvVars) error {
	otlpExporterConfig, otlpExporterEnvVars, err := otlpExporterBuilder.MakeConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to make otlp exporter config: %w", err)
	}

	receiver := makeFileLogReceiver(*pipeline)

	maps.Copy(envVars, otlpExporterEnvVars)

	otlpExporterID := otlpexporter.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
	cfg.Exporters[otlpExporterID] = Exporter{OTLP: otlpExporterConfig}

	otlpReceiverID := fmt.Sprintf("filelog/%s", pipeline.Name)
	cfg.Receivers[otlpReceiverID] = Receiver{FileLog: receiver}

	pipelineID := fmt.Sprintf("logs/%s", pipeline.Name)
	cfg.Service.Pipelines[pipelineID] = makePipelineConfig(otlpReceiverID, otlpExporterID)

	cfg.Service.Extensions = []string{"health_check", "pprof", "file_storage"}

	return nil
}

// Each pipeline will have one receiver and one exporter
func makePipelineConfig(receiverIDs, exporterIDs string) config.Pipeline {
	return config.Pipeline{
		Receivers: []string{receiverIDs},
		Processors: []string{
			"memory_limiter",
			"transform/set-instrumentation-scope-runtime",
			"k8sattributes",
			"resource/insert-cluster-attributes",
			"service_enrichment",
			"resource/drop-kyma-attributes",
		},
		Exporters: []string{exporterIDs},
	}
}
