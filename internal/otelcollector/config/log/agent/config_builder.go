package agent

import (
	"context"
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/processors"
	"maps"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sort"

	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

type BuilderConfig struct {
	GatewayOTLPServiceName types.NamespacedName
}
type Builder struct {
	Reader client.Reader
	Config BuilderConfig
}

type BuildOptions struct {
	InstrumentationScopeVersion string
	AgentNamespace              string
	ClusterName                 string
	CloudProvider               string
	Enrichments                 processors.Enrichments
}

// Currently the queue is disabled. So set the size to 0
const queueSize = 0

func (b *Builder) Build(logPipelines []telemetryv1alpha1.LogPipeline, opts BuildOptions) (*Config, otlpexporter.EnvVars, error) {
	logService := config.DefaultService(makePipelinesConfig())
	// Overwrite the extension from default service name
	logService.Extensions = []string{"health_check", "pprof", "file_storage"}

	cfg := &Config{
		Service:    logService,
		Extensions: makeExtensionsConfig(),

		// have filelog receiver for each log pipeline
		Receivers: makeReceivers(logPipelines, opts),
		// Add k8s attributes and resource/insert clustername
		Processors: makeProcessorsConfig(opts),
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

func makePipelinesConfig() config.Pipelines {
	pipelinesConfig := make(config.Pipelines)
	pipelinesConfig["logs"] = config.Pipeline{
		Receivers:  []string{"filelog"},
		Processors: []string{"memory_limiter", "transform/set-instrumentation-scope-runtime"},
		Exporters:  []string{"otlp"},
	}

	return pipelinesConfig
}

// addComponentsForLogPipeline enriches a Config (exporters, processors, etc.) with components for a given telemetryv1alpha1.LogPipeline.
func addComponentsForLogPipeline(ctx context.Context, otlpExporterBuilder *otlpexporter.ConfigBuilder, pipeline *telemetryv1alpha1.LogPipeline, cfg *Config, envVars otlpexporter.EnvVars) error {
	otlpExporterConfig, otlpExporterEnvVars, err := otlpExporterBuilder.MakeConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to make otlp exporter config: %w", err)
	}

	receiver := makeReciver(pipeline.Name, opts)

	maps.Copy(envVars, otlpExporterEnvVars)

	otlpExporterID := otlpexporter.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
	cfg.Exporters[otlpExporterID] = Exporter{OTLP: otlpExporterConfig}

	pipelineID := fmt.Sprintf("logs/%s", pipeline.Name)
	cfg.Service.Pipelines[pipelineID] = makePipelineConfig(otlpExporterID)

	return nil
}

func makePipelineConfig(exporterIDs ...string) config.Pipeline {
	sort.Strings(exporterIDs)

	return config.Pipeline{
		Receivers: []string{"otlp"},
		Processors: []string{
			"memory_limiter",
			"transform/set-instrumentation-scope-runtime",
			"k8sattributes",
			"resource/insert-cluster-attributes",
			"resource/drop-kyma-attributes",
		},
		Exporters: exporterIDs,
	}
}
