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

type BuildComponentFunc = func(ctx context.Context, tp *telemetryv1alpha1.TracePipeline) error

// ComponentConfigFunc creates the configuration for a component
type ComponentConfigFunc func(tp *telemetryv1alpha1.TracePipeline) any

type ComponentConfigFuncWithEnvVars func(ctx context.Context, tp *telemetryv1alpha1.TracePipeline) (any, common.EnvVars, error)

// ConditionFunc determines if a component should be added to the pipeline
type ConditionFunc func(ctx context.Context, tp *telemetryv1alpha1.TracePipeline) bool

// ComponentIDFunc determines the ID of a component
type ComponentIDFunc func(*telemetryv1alpha1.TracePipeline) string

func StaticComponentIDFunc(componentID string) ComponentIDFunc {
	return func(*telemetryv1alpha1.TracePipeline) string {
		return componentID
	}
}

// WithReceiver creates a decorator for adding receivers
func (b *Builder) WithReceiver(componentIDFunc ComponentIDFunc, configFunc ComponentConfigFunc) BuildComponentFunc {
	return func(ctx context.Context, tp *telemetryv1alpha1.TracePipeline) error {
		componentID := componentIDFunc(tp)

		config := configFunc(tp)
		if config == nil {
			// If no config is provided, skip adding the receiver
			return nil
		}

		if _, found := b.config.Receivers[componentID]; !found {
			b.config.Receivers[componentID] = config
		}

		pipelineID := formatTracePipelineID(tp.Name)
		pipeline := b.config.Service.Pipelines[pipelineID]
		pipeline.Receivers = append(pipeline.Receivers, componentID)
		b.config.Service.Pipelines[pipelineID] = pipeline

		return nil
	}
}

// WithProcessor creates a decorator for adding processors
func (b *Builder) WithProcessor(componentIDFunc ComponentIDFunc, configFunc ComponentConfigFunc) BuildComponentFunc {
	return func(ctx context.Context, tp *telemetryv1alpha1.TracePipeline) error {
		componentID := componentIDFunc(tp)

		config := configFunc(tp)
		if config == nil {
			// If no config is provided, skip adding the processor
			return nil
		}

		if _, found := b.config.Processors[componentID]; !found {
			config := configFunc(tp)
			b.config.Processors[componentID] = config
		}

		pipelineID := formatTracePipelineID(tp.Name)
		pipeline := b.config.Service.Pipelines[pipelineID]
		pipeline.Processors = append(pipeline.Processors, componentID)
		b.config.Service.Pipelines[pipelineID] = pipeline

		return nil
	}
}

// WithExporter creates a decorator for adding exporters
func (b *Builder) WithExporter(componentIDFunc ComponentIDFunc, configFunc ComponentConfigFuncWithEnvVars) BuildComponentFunc {
	return func(ctx context.Context, tp *telemetryv1alpha1.TracePipeline) error {
		componentID := componentIDFunc(tp)

		config, envVars, err := configFunc(ctx, tp)
		if err != nil {
			return fmt.Errorf("failed to create exporter config: %w", err)
		}

		if config == nil {
			// If no config is provided, skip adding the exporter
			return nil
		}

		b.config.Exporters[componentID] = config
		maps.Copy(b.envVars, envVars)

		pipelineID := formatTracePipelineID(tp.Name)
		pipeline := b.config.Service.Pipelines[pipelineID]
		pipeline.Exporters = append(pipeline.Exporters, componentID)
		b.config.Service.Pipelines[pipelineID] = pipeline

		return nil
	}
}

func (b *Builder) addServicePipeline(ctx context.Context, pipeline *telemetryv1alpha1.TracePipeline, fs ...BuildComponentFunc) error {
	for _, f := range fs {
		if err := f(ctx, pipeline); err != nil {
			return fmt.Errorf("failed to add component: %w", err)
		}
	}

	return nil
}

func (b *Builder) addOTLPReceiver() BuildComponentFunc {
	return b.WithReceiver(
		StaticComponentIDFunc(common.ComponentIDOTLPReceiver),
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
func (b *Builder) addMemoryLimiterProcessor() BuildComponentFunc {
	return b.WithProcessor(
		StaticComponentIDFunc(common.ComponentIDMemoryLimiterProcessor),
		func(tp *telemetryv1alpha1.TracePipeline) any {
			return &common.MemoryLimiter{
				CheckInterval:        "1s",
				LimitPercentage:      75,
				SpikeLimitPercentage: 15,
			}
		},
	)
}

func (b *Builder) addK8sAttributesProcessor(opts BuildOptions) BuildComponentFunc {
	return b.WithProcessor(
		StaticComponentIDFunc(common.ComponentIDK8sAttributesProcessor),
		func(tp *telemetryv1alpha1.TracePipeline) any {
			return common.K8sAttributesProcessorConfig(opts.Enrichments)
		},
	)
}

func (b *Builder) addIstioNoiseFilterProcessor() BuildComponentFunc {
	return b.WithProcessor(
		StaticComponentIDFunc(common.ComponentIDIstioNoiseFilterProcessor),
		func(tp *telemetryv1alpha1.TracePipeline) any {
			return &common.IstioNoiseFilterProcessor{}
		},
	)
}

func (b *Builder) addInsertClusterAttributesProcessor(opts BuildOptions) BuildComponentFunc {
	return b.WithProcessor(
		StaticComponentIDFunc(common.ComponentIDInsertClusterAttributesProcessor),
		func(tp *telemetryv1alpha1.TracePipeline) any {
			return common.InsertClusterAttributesProcessorConfig(
				opts.ClusterName, opts.ClusterUID, opts.CloudProvider,
			)
		},
	)
}

func (b *Builder) addServiceEnrichmentProcessor() BuildComponentFunc {
	return b.WithProcessor(
		StaticComponentIDFunc(common.ComponentIDServiceEnrichmentProcessor),
		func(tp *telemetryv1alpha1.TracePipeline) any {
			return common.ResolveServiceNameConfig()
		},
	)
}

func (b *Builder) addDropKymaAttributesProcessor() BuildComponentFunc {
	return b.WithProcessor(
		StaticComponentIDFunc(common.ComponentIDDropKymaAttributesProcessor),
		func(tp *telemetryv1alpha1.TracePipeline) any {
			return common.DropKymaAttributesProcessorConfig()
		},
	)
}

// addUserDefinedTransformProcessor handles user-defined transform processors with dynamic component IDs
func (b *Builder) addUserDefinedTransformProcessor() BuildComponentFunc {
	return b.WithProcessor(
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
func (b *Builder) addBatchProcessor() BuildComponentFunc {
	return b.WithProcessor(
		StaticComponentIDFunc(common.ComponentIDBatchProcessor),
		func(_ *telemetryv1alpha1.TracePipeline) any {
			return &common.BatchProcessor{
				SendBatchSize:    512,
				Timeout:          "10s",
				SendBatchMaxSize: 512,
			}
		},
	)
}

func (b *Builder) addOTLPExporter(queueSize int) BuildComponentFunc {
	return b.WithExporter(
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
				return nil, nil, fmt.Errorf("failed to make otlp exporter config: %w", err)
			}

			return otlpExporterConfig, otlpExporterEnvVars, nil
		},
	)
}

func formatTracePipelineID(pipelineName string) string {
	return fmt.Sprintf("traces/%s", pipelineName)
}

func formatUserDefinedTransformProcessorID(pipeline *telemetryv1alpha1.TracePipeline) string {
	return fmt.Sprintf("transform/user-defined-%s", pipeline.Name)
}

func formatOTLPExporterID(pipeline *telemetryv1alpha1.TracePipeline) string {
	return common.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
}
