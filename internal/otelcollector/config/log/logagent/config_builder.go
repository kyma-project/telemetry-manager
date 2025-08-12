package logagent

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

	config  *Config
	envVars otlpexporter.EnvVars
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
	b.config = b.baseConfig(opts)
	b.envVars = make(otlpexporter.EnvVars)

	// Iterate over each LogPipeline CR and enrich the config with pipeline-specific components
	for i := range logPipelines {
		pipeline := logPipelines[i]

		if err := b.addComponentsForLogPipeline(ctx, &pipeline, queueSize); err != nil {
			return nil, nil, err
		}

		b.addServicePipelines(&pipeline)
	}

	return b.config, b.envVars, nil
}

// baseConfig creates the static/global base configuration for the log agent collector.
// Pipeline-specific components are added later via addComponentsForLogPipeline method.
func (b *Builder) baseConfig(opts BuildOptions) *Config {
	service := config.DefaultService(make(config.Pipelines))
	service.Extensions = append(service.Extensions, "file_storage")

	return &Config{
		Service:    service,
		Extensions: extensionsConfig(),
		Receivers:  make(Receivers),
		Processors: processorsConfig(opts),
		Exporters:  make(Exporters),
	}
}

// addComponentsForLogPipeline enriches a Config (exporters, processors, etc.) with components for a given telemetryv1alpha1.LogPipeline.
func (b *Builder) addComponentsForLogPipeline(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline, queueSize int) error {
	b.addFileLogReceiver(pipeline)
	b.addTransformProcessors(pipeline)

	return b.addOTLPExporter(ctx, pipeline, queueSize)
}

func (b *Builder) addFileLogReceiver(pipeline *telemetryv1alpha1.LogPipeline) {
	receiver := fileLogReceiverConfig(*pipeline)

	receiverID := formatFileLogReceiverID(pipeline.Name)
	b.config.Receivers[receiverID] = Receiver{FileLog: receiver}
}

func (b *Builder) addTransformProcessors(pipeline *telemetryv1alpha1.LogPipeline) {
	if len(pipeline.Spec.Transforms) == 0 {
		return
	}

	transformStatements := config.TransformSpecsToProcessorStatements(pipeline.Spec.Transforms)
	transformProcessor := config.LogTransformProcessor(transformStatements)

	processorID := formatUserDefinedTransformProcessorID(pipeline.Name)
	b.config.Processors.Dynamic[processorID] = transformProcessor
}

func (b *Builder) addOTLPExporter(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline, queueSize int) error {
	otlpExporterBuilder := otlpexporter.NewConfigBuilder(
		b.Reader,
		pipeline.Spec.Output.OTLP,
		pipeline.Name,
		queueSize,
		otlpexporter.SignalTypeLog,
	)

	otlpExporterConfig, otlpExporterEnvVars, err := otlpExporterBuilder.OTLPExporterConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to make otlp exporter config: %w", err)
	}

	maps.Copy(b.envVars, otlpExporterEnvVars)

	otlpExporterID := otlpexporter.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
	b.config.Exporters[otlpExporterID] = Exporter{OTLP: otlpExporterConfig}

	return nil
}
