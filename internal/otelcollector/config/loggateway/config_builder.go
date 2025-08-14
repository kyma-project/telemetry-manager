package loggateway

import (
	"context"
	"fmt"
	"maps"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
)

const (
	maxQueueSize = 256 // Maximum number of batches kept in memory before dropping
)

type Builder struct {
	Reader client.Reader

	config  *Config
	envVars common.EnvVars
}

type BuildOptions struct {
	ClusterName   string
	ClusterUID    string
	CloudProvider string
	Enrichments   *operatorv1alpha1.EnrichmentSpec
	ModuleVersion string
}

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.LogPipeline, opts BuildOptions) (*Config, common.EnvVars, error) {
	b.config = b.baseConfig(opts)
	b.envVars = make(common.EnvVars)

	// Iterate over each LogPipeline CR and enrich the config with pipeline-specific components
	queueSize := maxQueueSize / len(pipelines)

	for i := range pipelines {
		pipeline := pipelines[i]

		if err := b.addComponentsForLogPipeline(ctx, &pipeline, queueSize); err != nil {
			return nil, nil, err
		}

		b.addServicePipelines(&pipeline)
	}

	return b.config, b.envVars, nil
}

// baseConfig creates the static/global base configuration for the log gateway collector.
// Pipeline-specific components are added later via addComponentsForLogPipeline method.
func (b *Builder) baseConfig(opts BuildOptions) *Config {
	return &Config{
		Base: common.Base{
			Service:    common.ServiceConfig(),
			Extensions: common.ExtensionsConfig(),
		},
		Receivers:  receiversConfig(),
		Processors: processorsConfig(opts),
		Exporters:  make(Exporters),
	}
}

func receiversConfig() Receivers {
	return Receivers{
		OTLP: common.OTLPReceiver{
			Protocols: common.ReceiverProtocols{
				HTTP: common.Endpoint{
					Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.OTLPHTTP),
				},
				GRPC: common.Endpoint{
					Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.OTLPGRPC),
				},
			},
		},
	}
}

// addComponentsForLogPipeline enriches a Config (exporters, processors, etc.) with components for a given telemetryv1alpha1.LogPipeline.
func (b *Builder) addComponentsForLogPipeline(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline, queueSize int) error {
	b.addNamespaceFilter(pipeline)
	b.addInputSourceFilters(pipeline)
	b.addUserDefinedTransformProcessor(pipeline)

	return b.addOTLPExporter(ctx, pipeline, queueSize)
}

func (b *Builder) addNamespaceFilter(pipeline *telemetryv1alpha1.LogPipeline) {
	otlpInput := pipeline.Spec.Input.OTLP
	if otlpInput == nil || otlpInput.Disabled {
		// no namespace filter needed
		return
	}

	if b.config.Processors.Dynamic == nil {
		b.config.Processors.Dynamic = make(map[string]any)
	}

	if shouldFilterByNamespace(otlpInput.Namespaces) {
		processorID := formatNamespaceFilterID(pipeline.Name)
		b.config.Processors.Dynamic[processorID] = namespaceFilterProcessorConfig(otlpInput.Namespaces)
	}
}

func (b *Builder) addInputSourceFilters(pipeline *telemetryv1alpha1.LogPipeline) {
	input := pipeline.Spec.Input
	if !logpipelineutils.IsOTLPInputEnabled(input) {
		b.config.Processors.DropIfInputSourceOTLP = dropIfInputSourceOTLPProcessorConfig()
	}
}

func (b *Builder) addUserDefinedTransformProcessor(pipeline *telemetryv1alpha1.LogPipeline) {
	if len(pipeline.Spec.Transforms) == 0 {
		return
	}

	transformStatements := common.TransformSpecsToProcessorStatements(pipeline.Spec.Transforms)
	transformProcessor := common.LogTransformProcessorConfig(transformStatements)

	processorID := formatUserDefinedTransformProcessorID(pipeline.Name)
	b.config.Processors.Dynamic[processorID] = transformProcessor
}

func (b *Builder) addOTLPExporter(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline, queueSize int) error {
	otlpExporterBuilder := common.NewOTLPExporterConfigBuilder(
		b.Reader,
		pipeline.Spec.Output.OTLP,
		pipeline.Name,
		queueSize,
		common.SignalTypeLog,
	)

	otlpExporterConfig, otlpExporterEnvVars, err := otlpExporterBuilder.OTLPExporterConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to make otlp exporter config: %w", err)
	}

	maps.Copy(b.envVars, otlpExporterEnvVars)

	otlpExporterID := formatOTLPExporterID(pipeline)
	b.config.Exporters[otlpExporterID] = Exporter{OTLP: otlpExporterConfig}

	return nil
}
