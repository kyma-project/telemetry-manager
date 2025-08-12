package tracegateway

import (
	"context"
	"fmt"
	"maps"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
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
}

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.TracePipeline, opts BuildOptions) (*Config, common.EnvVars, error) {
	b.config = b.baseConfig(opts)
	b.envVars = make(common.EnvVars)

	// Iterate over each TracePipeline CR and enrich the config with pipeline-specific components
	queueSize := maxQueueSize / len(pipelines)

	for i := range pipelines {
		pipeline := pipelines[i]

		if err := b.addComponentsForTracePipeline(ctx, &pipeline, queueSize); err != nil {
			return nil, nil, err
		}

		b.addServicePipelines(&pipeline)
	}

	return b.config, b.envVars, nil
}

// baseConfig creates the static/global base configuration for the trace gateway collector.
// Pipeline-specific components are added later via addComponentsForTracePipeline method.
func (b *Builder) baseConfig(opts BuildOptions) *Config {
	return &Config{
		Base: common.Base{
			Service:    common.DefaultService(make(common.Pipelines)),
			Extensions: common.DefaultExtensions(),
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

// addComponentsForTracePipeline enriches a Config (exporters, processors, etc.) with components for a given telemetryv1alpha1.TracePipeline.
func (b *Builder) addComponentsForTracePipeline(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline, queueSize int) error {
	b.addUserDefinedTransformProcessor(pipeline)
	return b.addOTLPExporter(ctx, pipeline, queueSize)
}

func (b *Builder) addUserDefinedTransformProcessor(pipeline *telemetryv1alpha1.TracePipeline) {
	if len(pipeline.Spec.Transforms) == 0 {
		return
	}

	if b.config.Processors.Dynamic == nil {
		b.config.Processors.Dynamic = make(map[string]any)
	}

	transformStatements := common.TransformSpecsToProcessorStatements(pipeline.Spec.Transforms)
	transformProcessor := common.TraceTransformProcessor(transformStatements)

	processorID := formatUserDefinedTransformProcessorID(pipeline.Name)
	b.config.Processors.Dynamic[processorID] = transformProcessor
}

func (b *Builder) addOTLPExporter(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline, queueSize int) error {
	otlpExporterBuilder := common.NewConfigBuilder(
		b.Reader,
		pipeline.Spec.Output.OTLP,
		pipeline.Name,
		queueSize,
		common.SignalTypeTrace,
	)

	otlpExporterConfig, otlpExporterEnvVars, err := otlpExporterBuilder.OTLPExporterConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to make otlp exporter config: %w", err)
	}

	maps.Copy(b.envVars, otlpExporterEnvVars)

	otlpExporterID := common.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
	b.config.Exporters[otlpExporterID] = Exporter{OTLP: otlpExporterConfig}

	return nil
}
