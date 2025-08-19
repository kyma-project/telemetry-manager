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

// buildComponentFunc defines a function type for building components in the telemetry configuration
type buildComponentFunc func(ctx context.Context, tp *telemetryv1alpha1.TracePipeline) error

// componentConfigFunc creates the configuration for a component (receiver or processor)
type componentConfigFunc func(tp *telemetryv1alpha1.TracePipeline) any

// exporterComponentConfigFunc creates the configuration for an exporter component
// creating exporters is different from receivers and processors, as it makes an API server call to resolve the reference secrets
// and returns the configuration along with environment variables needed for the exporter
type exporterComponentConfigFunc func(ctx context.Context, tp *telemetryv1alpha1.TracePipeline) (any, common.EnvVars, error)

// componentIDFunc determines the ID of a component
type componentIDFunc func(*telemetryv1alpha1.TracePipeline) string

// staticComponentID returns a ComponentIDFunc that always returns the same component ID independent of the TracePipeline
func staticComponentID(componentID string) componentIDFunc {
	return func(*telemetryv1alpha1.TracePipeline) string {
		return componentID
	}
}

func (b *Builder) addServicePipeline(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline, fs ...buildComponentFunc) error {
	// Add an empty pipeline to the config
	pipelineID := formatTraceServicePipelineID(pipeline)
	b.config.Service.Pipelines[pipelineID] = common.Pipeline{}

	for _, f := range fs {
		if err := f(ctx, pipeline); err != nil {
			return fmt.Errorf("failed to add component: %w", err)
		}
	}

	return nil
}

// withReceiver creates a decorator for adding receivers
func (b *Builder) withReceiver(componentIDFunc componentIDFunc, configFunc componentConfigFunc) buildComponentFunc {
	return func(ctx context.Context, tp *telemetryv1alpha1.TracePipeline) error {
		config := configFunc(tp)
		if config == nil {
			// If no config is provided, skip adding the receiver
			return nil
		}

		componentID := componentIDFunc(tp)
		if _, found := b.config.Receivers[componentID]; !found {
			b.config.Receivers[componentID] = config
		}

		if len(b.config.Service.Pipelines) == 0 {
			panic("no service pipelines found in config, cannot add receiver")
		}

		pipelineID := formatTraceServicePipelineID(tp)
		pipeline := b.config.Service.Pipelines[pipelineID]
		pipeline.Receivers = append(pipeline.Receivers, componentID)
		b.config.Service.Pipelines[pipelineID] = pipeline

		return nil
	}
}

// withProcessor creates a decorator for adding processors
func (b *Builder) withProcessor(componentIDFunc componentIDFunc, configFunc componentConfigFunc) buildComponentFunc {
	return func(ctx context.Context, tp *telemetryv1alpha1.TracePipeline) error {
		config := configFunc(tp)
		if config == nil {
			// If no config is provided, skip adding the processor
			return nil
		}

		componentID := componentIDFunc(tp)
		if _, found := b.config.Processors[componentID]; !found {
			config := configFunc(tp)
			b.config.Processors[componentID] = config
		}

		pipelineID := formatTraceServicePipelineID(tp)
		pipeline := b.config.Service.Pipelines[pipelineID]
		pipeline.Processors = append(pipeline.Processors, componentID)
		b.config.Service.Pipelines[pipelineID] = pipeline

		return nil
	}
}

// withExporter creates a decorator for adding exporters
func (b *Builder) withExporter(componentIDFunc componentIDFunc, configFunc exporterComponentConfigFunc) buildComponentFunc {
	return func(ctx context.Context, tp *telemetryv1alpha1.TracePipeline) error {
		config, envVars, err := configFunc(ctx, tp)
		if err != nil {
			return fmt.Errorf("failed to create exporter config: %w", err)
		}

		if config == nil {
			// If no config is provided, skip adding the exporter
			return nil
		}

		componentID := componentIDFunc(tp)
		b.config.Exporters[componentID] = config
		maps.Copy(b.envVars, envVars)

		pipelineID := formatTraceServicePipelineID(tp)
		pipeline := b.config.Service.Pipelines[pipelineID]
		pipeline.Exporters = append(pipeline.Exporters, componentID)
		b.config.Service.Pipelines[pipelineID] = pipeline

		return nil
	}
}

func (b *Builder) addOTLPReceiver() buildComponentFunc {
	return b.withReceiver(
		staticComponentID(common.ComponentIDOTLPReceiver),
		func(tp *telemetryv1alpha1.TracePipeline) any {
			return &common.OTLPReceiver{
				Protocols: common.ReceiverProtocols{
					HTTP: common.Endpoint{
						Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.OTLPHTTP),
					},
					GRPC: common.Endpoint{
						Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.OTLPGRPC),
					},
				},
			}
		},
	)
}

//nolint:mnd // hardcoded values
func (b *Builder) addMemoryLimiterProcessor() buildComponentFunc {
	return b.withProcessor(
		staticComponentID(common.ComponentIDMemoryLimiterProcessor),
		func(tp *telemetryv1alpha1.TracePipeline) any {
			return &common.MemoryLimiter{
				CheckInterval:        "1s",
				LimitPercentage:      75,
				SpikeLimitPercentage: 15,
			}
		},
	)
}

func (b *Builder) addK8sAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.withProcessor(
		staticComponentID(common.ComponentIDK8sAttributesProcessor),
		func(tp *telemetryv1alpha1.TracePipeline) any {
			return common.K8sAttributesProcessorConfig(opts.Enrichments)
		},
	)
}

func (b *Builder) addIstioNoiseFilterProcessor() buildComponentFunc {
	return b.withProcessor(
		staticComponentID(common.ComponentIDIstioNoiseFilterProcessor),
		func(tp *telemetryv1alpha1.TracePipeline) any {
			return &common.IstioNoiseFilterProcessor{}
		},
	)
}

func (b *Builder) addInsertClusterAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.withProcessor(
		staticComponentID(common.ComponentIDInsertClusterAttributesProcessor),
		func(tp *telemetryv1alpha1.TracePipeline) any {
			return common.InsertClusterAttributesProcessorConfig(
				opts.ClusterName, opts.ClusterUID, opts.CloudProvider,
			)
		},
	)
}

func (b *Builder) addServiceEnrichmentProcessor() buildComponentFunc {
	return b.withProcessor(
		staticComponentID(common.ComponentIDServiceEnrichmentProcessor),
		func(tp *telemetryv1alpha1.TracePipeline) any {
			return common.ResolveServiceNameConfig()
		},
	)
}

func (b *Builder) addDropKymaAttributesProcessor() buildComponentFunc {
	return b.withProcessor(
		staticComponentID(common.ComponentIDDropKymaAttributesProcessor),
		func(tp *telemetryv1alpha1.TracePipeline) any {
			return common.DropKymaAttributesProcessorConfig()
		},
	)
}

// addUserDefinedTransformProcessor handles user-defined transform processors with dynamic component IDs
func (b *Builder) addUserDefinedTransformProcessor() buildComponentFunc {
	return b.withProcessor(
		formatUserDefinedTransformProcessorID,
		func(tp *telemetryv1alpha1.TracePipeline) any {
			if len(tp.Spec.Transforms) == 0 {
				return nil // No transforms, no processor needed
			}

			transformStatements := common.TransformSpecsToProcessorStatements(tp.Spec.Transforms)
			transformProcessor := common.TraceTransformProcessorConfig(transformStatements)

			return transformProcessor
		},
	)
}

//nolint:mnd // hardcoded values
func (b *Builder) addBatchProcessor() buildComponentFunc {
	return b.withProcessor(
		staticComponentID(common.ComponentIDBatchProcessor),
		func(_ *telemetryv1alpha1.TracePipeline) any {
			return &common.BatchProcessor{
				SendBatchSize:    512,
				Timeout:          "10s",
				SendBatchMaxSize: 512,
			}
		},
	)
}

func (b *Builder) addOTLPExporter(queueSize int) buildComponentFunc {
	return b.withExporter(
		formatOTLPExporterID,
		func(ctx context.Context, tp *telemetryv1alpha1.TracePipeline) (any, common.EnvVars, error) {
			otlpExporterBuilder := common.NewOTLPExporterConfigBuilder(
				b.Reader,
				tp.Spec.Output.OTLP,
				tp.Name,
				queueSize,
				common.SignalTypeTrace,
			)

			otlpExporterConfig, otlpExporterEnvVars, err := otlpExporterBuilder.OTLPExporterConfig(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create otlp exporter config: %w", err)
			}

			return otlpExporterConfig, otlpExporterEnvVars, nil
		},
	)
}

func formatTraceServicePipelineID(tp *telemetryv1alpha1.TracePipeline) string {
	return fmt.Sprintf("traces/%s", tp.Name)
}

func formatUserDefinedTransformProcessorID(tp *telemetryv1alpha1.TracePipeline) string {
	return fmt.Sprintf("transform/user-defined-%s", tp.Name)
}

func formatOTLPExporterID(tp *telemetryv1alpha1.TracePipeline) string {
	return common.ExporterID(tp.Spec.Output.OTLP.Protocol, tp.Name)
}
