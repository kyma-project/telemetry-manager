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
	b.config = &Config{
		Base:       common.BaseConfig(),
		Receivers:  make(map[string]any),
		Processors: make(map[string]any),
		Exporters:  make(map[string]any),
	}
	b.envVars = make(common.EnvVars)

	// Iterate over each TracePipeline CR and enrich the config with pipeline-specific components
	queueSize := maxQueueSize / len(pipelines)

	for i := range pipelines {
		if err := b.addServicePipeline(ctx, &pipelines[i],
			b.addOTLPReceiver(),
			b.addMemoryLimiterProcessor(),
			b.addK8sAttributesProcessor(opts),
			b.addIstioNoiseFilterProcessor(),
			b.addInsertClusterAttributesProcessor(opts),
			b.addServiceEnrichmentProcessor(),
			b.addDropKymaAttributesProcessor(),
			b.addUserDefinedTransformProcessor(),
			b.addBatchProcessor(),
			b.addOTLPExporter(queueSize),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add service pipeline: %w", err)
		}
	}

	return b.config, b.envVars, nil
}

type addComponentFunc = func(ctx context.Context, tp *telemetryv1alpha1.TracePipeline) error

func (b *Builder) addServicePipeline(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline, fs ...addComponentFunc) error {
	for _, f := range fs {
		if err := f(ctx, pipeline); err != nil {
			return fmt.Errorf("failed to add component: %w", err)
		}
	}

	return nil
}

func (b *Builder) addOTLPReceiver() addComponentFunc {
	return func(ctx context.Context, tp *telemetryv1alpha1.TracePipeline) error {
		if _, found := b.config.Receivers[common.ComponentIDOTLPReceiver]; !found {
			b.config.Receivers[common.ComponentIDOTLPReceiver] = &common.OTLPReceiver{
				Protocols: common.ReceiverProtocols{
					HTTP: common.Endpoint{
						Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.OTLPHTTP),
					},
					GRPC: common.Endpoint{
						Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.OTLPGRPC),
					},
				},
			}
		}

		pipelineID := formatTracePipelineID(tp.Name)
		pipeline := b.config.Service.Pipelines[pipelineID]
		pipeline.Receivers = append(pipeline.Receivers, common.ComponentIDOTLPReceiver)

		return nil
	}
}

func (b *Builder) addMemoryLimiterProcessor() addComponentFunc {
	return func(ctx context.Context, tp *telemetryv1alpha1.TracePipeline) error {
		if _, found := b.config.Processors[common.ComponentIDMemoryLimiterProcessor]; !found {
			b.config.Processors[common.ComponentIDMemoryLimiterProcessor] = &common.MemoryLimiter{
				CheckInterval:        "1s",
				LimitPercentage:      75,
				SpikeLimitPercentage: 15,
			}
		}

		pipelineID := formatTracePipelineID(tp.Name)
		pipeline := b.config.Service.Pipelines[pipelineID]
		pipeline.Processors = append(pipeline.Processors, common.ComponentIDMemoryLimiterProcessor)

		return nil
	}
}

func (b *Builder) addK8sAttributesProcessor(opts BuildOptions) addComponentFunc {
	return func(ctx context.Context, tp *telemetryv1alpha1.TracePipeline) error {
		if _, found := b.config.Processors[common.ComponentIDK8sAttributesProcessor]; !found {
			b.config.Processors[common.ComponentIDK8sAttributesProcessor] = common.K8sAttributesProcessorConfig(opts.Enrichments)
		}

		pipelineID := formatTracePipelineID(tp.Name)
		pipeline := b.config.Service.Pipelines[pipelineID]
		pipeline.Processors = append(pipeline.Processors, common.ComponentIDK8sAttributesProcessor)

		return nil
	}
}

func (b *Builder) addIstioNoiseFilterProcessor() addComponentFunc {
	return func(ctx context.Context, tp *telemetryv1alpha1.TracePipeline) error {
		if _, found := b.config.Processors[common.ComponentIDIstioNoiseFilterProcessor]; !found {
			b.config.Processors[common.ComponentIDIstioNoiseFilterProcessor] = &common.IstioNoiseFilterProcessor{}
		}

		pipelineID := formatTracePipelineID(tp.Name)
		pipeline := b.config.Service.Pipelines[pipelineID]
		pipeline.Processors = append(pipeline.Processors, common.ComponentIDIstioNoiseFilterProcessor)

		return nil
	}
}

func (b *Builder) addInsertClusterAttributesProcessor(opts BuildOptions) addComponentFunc {
	return func(ctx context.Context, tp *telemetryv1alpha1.TracePipeline) error {
		if _, found := b.config.Processors[common.ComponentIDInsertClusterAttributesProcessor]; !found {
			b.config.Processors[common.ComponentIDInsertClusterAttributesProcessor] = common.InsertClusterAttributesProcessorConfig(
				opts.ClusterName, opts.ClusterUID, opts.CloudProvider,
			)
		}

		pipelineID := formatTracePipelineID(tp.Name)
		pipeline := b.config.Service.Pipelines[pipelineID]
		pipeline.Processors = append(pipeline.Processors, common.ComponentIDInsertClusterAttributesProcessor)

		return nil
	}
}

func (b *Builder) addServiceEnrichmentProcessor() addComponentFunc {
	return func(ctx context.Context, tp *telemetryv1alpha1.TracePipeline) error {
		if _, found := b.config.Processors[common.ComponentIDServiceEnrichmentProcessor]; !found {
			b.config.Processors[common.ComponentIDServiceEnrichmentProcessor] = common.ResolveServiceNameConfig()
		}

		pipelineID := formatTracePipelineID(tp.Name)
		pipeline := b.config.Service.Pipelines[pipelineID]
		pipeline.Processors = append(pipeline.Processors, common.ComponentIDServiceEnrichmentProcessor)

		return nil
	}
}

func (b *Builder) addDropKymaAttributesProcessor() addComponentFunc {
	return func(ctx context.Context, tp *telemetryv1alpha1.TracePipeline) error {
		if _, found := b.config.Processors[common.ComponentIDDropKymaAttributesProcessor]; !found {
			b.config.Processors[common.ComponentIDDropKymaAttributesProcessor] = common.DropKymaAttributesProcessorConfig()
		}

		pipelineID := formatTracePipelineID(tp.Name)
		pipeline := b.config.Service.Pipelines[pipelineID]
		pipeline.Processors = append(pipeline.Processors, common.ComponentIDDropKymaAttributesProcessor)

		return nil
	}
}

func (b *Builder) addUserDefinedTransformProcessor() addComponentFunc {
	return func(ctx context.Context, tp *telemetryv1alpha1.TracePipeline) error {
		if len(tp.Spec.Transforms) == 0 {
			return nil
		}

		processorID := formatUserDefinedTransformProcessorID(tp.Name)

		if _, found := b.config.Processors[formatUserDefinedTransformProcessorID(tp.Name)]; !found {
			transformStatements := common.TransformSpecsToProcessorStatements(tp.Spec.Transforms)
			transformProcessor := common.TraceTransformProcessorConfig(transformStatements)

			b.config.Processors[processorID] = transformProcessor
		}

		pipelineID := formatTracePipelineID(tp.Name)
		pipeline := b.config.Service.Pipelines[pipelineID]
		pipeline.Processors = append(pipeline.Processors, processorID)

		return nil
	}
}

func (b *Builder) addBatchProcessor() addComponentFunc {
	return func(ctx context.Context, tp *telemetryv1alpha1.TracePipeline) error {
		if _, found := b.config.Processors[common.ComponentIDBatchProcessor]; !found {
			b.config.Processors[common.ComponentIDBatchProcessor] = &common.BatchProcessor{
				SendBatchSize:    512,
				Timeout:          "10s",
				SendBatchMaxSize: 512,
			}
		}

		pipelineID := formatTracePipelineID(tp.Name)
		pipeline := b.config.Service.Pipelines[pipelineID]
		pipeline.Processors = append(pipeline.Processors, common.ComponentIDBatchProcessor)

		return nil
	}
}

func (b *Builder) addOTLPExporter(queueSize int) addComponentFunc {
	return func(ctx context.Context, tp *telemetryv1alpha1.TracePipeline) error {
		otlpExporterBuilder := common.NewOTLPExporterConfigBuilder(
			b.Reader,
			tp.Spec.Output.OTLP,
			tp.Name,
			queueSize,
			common.SignalTypeTrace,
		)

		otlpExporterConfig, otlpExporterEnvVars, err := otlpExporterBuilder.OTLPExporterConfig(ctx)
		if err != nil {
			return fmt.Errorf("failed to make otlp exporter config: %w", err)
		}

		maps.Copy(b.envVars, otlpExporterEnvVars)

		otlpExporterID := formatOTLPExporterID(tp)
		b.config.Exporters[otlpExporterID] = otlpExporterConfig

		pipelineID := formatTracePipelineID(tp.Name)
		pipeline := b.config.Service.Pipelines[pipelineID]
		pipeline.Exporters = append(pipeline.Exporters, otlpExporterID)

		return nil
	}
}

func formatTracePipelineID(pipelineName string) string {
	return fmt.Sprintf("traces/%s", pipelineName)
}

func formatUserDefinedTransformProcessorID(pipelineName string) string {
	return fmt.Sprintf("transform/user-defined-%s", pipelineName)
}

func formatOTLPExporterID(pipeline *telemetryv1alpha1.TracePipeline) string {
	return common.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
}
