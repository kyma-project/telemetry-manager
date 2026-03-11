package common

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// BuildComponentFunc defines a function type for building OpenTelemetry collector components.
// It can be chained together to construct telemetry pipelines.
type BuildComponentFunc[T any] func(ctx context.Context, pipeline T, pipelineID string) error

// ComponentIDFunc determines the unique identifier for a component.
type ComponentIDFunc[T any] func(pipeline T) string

// ComponentConfigFunc creates the configuration for a component (receiver or processor).
// Returns nil to skip the component for this pipeline.
type ComponentConfigFunc[T any] func(pipeline T) any

// ExporterComponentConfigFunc creates exporter configuration and environment variables.
// Unlike receivers/processors, exporters often need secret resolution.
type ExporterComponentConfigFunc[T any] func(ctx context.Context, pipeline T) (any, EnvVars, error)

// PipelineIDFunc determines the unique identifier for a service pipeline.
type PipelineIDFunc[T any] func(pipeline T) string

// ComponentBuilder provides common builder patterns for OpenTelemetry collector configurations.
// It can be embedded into specific config builders to provide reusable component management.
type ComponentBuilder[T any] struct {
	Collector *Config
	EnvVars   EnvVars
}

// AddServicePipeline creates and configures a complete telemetry pipeline by chaining component builders.
// It initializes an empty pipeline and then applies each BuildComponentFunc to add receivers, processors, and exporters.
//
// Example:
//
//	func (b *Builder) Build(ctx context.Context, lp *LogPipeline) (*Config, EnvVars, error) {
//	    if err := b.AddServicePipeline(ctx, lp, fmt.Sprintf("logs/%s", lp.Name),
//	        b.addOTLPReceiver(),
//	        b.addMemoryLimiterProcessor(),
//	        b.addBatchProcessor(),
//	        b.addOTLPExporter(),
//	    ); err != nil {
//	        return nil, err
//	    }
//	    return b.Collector, b.EnvVars, nil
//	}
func (cb *ComponentBuilder[T]) AddServicePipeline(ctx context.Context, pipeline T, pipelineID string, fs ...BuildComponentFunc[T]) error {
	cb.Collector.Service.Pipelines[pipelineID] = PipelineConfig{}

	for _, f := range fs {
		if err := f(ctx, pipeline, pipelineID); err != nil {
			return fmt.Errorf("failed to add component: %w", err)
		}
	}

	return nil
}

// AddReceiver creates a BuildComponentFunc that adds a receiver to the configuration.
// Receivers collect telemetry data from various sources.
// If configFunc returns nil, the receiver is skipped for that pipeline.
//
// Example:
//
//	func (b *Builder) addOTLPReceiver() BuildComponentFunc[*LogPipeline] {
//	    return b.AddReceiver(
//	        b.StaticComponentID[*LogPipeline]("otlp"),
//	        func(lp *LogPipeline) any {
//	            return &OTLPReceiverConfig{
//	                Protocols: ReceiverProtocols{
//	                    HTTP: Endpoint{Endpoint: fmt.Sprintf("${%s}:4318", EnvVarCurrentPodIP)},
//	                    GRPC: Endpoint{Endpoint: fmt.Sprintf("${%s}:4317", EnvVarCurrentPodIP)},
//	                },
//	            }
//	        },
//	    )
//	}
func (cb *ComponentBuilder[T]) AddReceiver(componentIDFunc ComponentIDFunc[T], configFunc ComponentConfigFunc[T]) BuildComponentFunc[T] {
	return func(ctx context.Context, pipeline T, pipelineID string) error {
		receiverConfig := configFunc(pipeline)
		if receiverConfig == nil {
			// If no config is provided, skip adding the receiver
			return nil
		}

		componentID := componentIDFunc(pipeline)

		receiversOrConnectors := cb.Collector.Receivers
		if isConnector(componentID) {
			receiversOrConnectors = cb.Collector.Connectors
		}

		if _, found := receiversOrConnectors[componentID]; !found {
			receiversOrConnectors[componentID] = receiverConfig
		}

		pipelineCfg := cb.Collector.Service.Pipelines[pipelineID]
		pipelineCfg.Receivers = append(pipelineCfg.Receivers, componentID)
		cb.Collector.Service.Pipelines[pipelineID] = pipelineCfg

		return nil
	}
}

// AddProcessor creates a BuildComponentFunc that adds a processor to the configuration.
// Processors transform, filter, or enrich telemetry data. Order matters in pipelines.
// If configFunc returns nil, the processor is skipped for that pipeline.
//
// Example:
//
//	func (b *Builder) addMemoryLimiterProcessor() BuildComponentFunc[*LogPipeline] {
//	    return b.AddProcessor(
//	        b.StaticComponentID[*LogPipeline]("memory_limiter"),
//	        func(lp *LogPipeline) any {
//	            return &MemoryLimiterConfig{
//	                CheckInterval:        "1s",
//	                LimitPercentage:      75,
//	                SpikeLimitPercentage: 15,
//	            }
//	        },
//	    )
//	}
func (cb *ComponentBuilder[T]) AddProcessor(componentIDFunc ComponentIDFunc[T], configFunc ComponentConfigFunc[T]) BuildComponentFunc[T] {
	return func(ctx context.Context, pipeline T, pipelineID string) error {
		processorConfig := configFunc(pipeline)
		if processorConfig == nil {
			// If no config is provided, skip adding the processor
			return nil
		}

		componentID := componentIDFunc(pipeline)
		if _, found := cb.Collector.Processors[componentID]; !found {
			cb.Collector.Processors[componentID] = processorConfig
		}

		servicePipeline := cb.Collector.Service.Pipelines[pipelineID]
		servicePipeline.Processors = append(servicePipeline.Processors, componentID)
		cb.Collector.Service.Pipelines[pipelineID] = servicePipeline

		return nil
	}
}

// AddExporter creates a BuildComponentFunc that adds an exporter to the configuration.
// Exporters send telemetry data to external systems and often require secret resolution.
// If configFunc returns nil, the exporter is skipped for that pipeline.
//
// Example:
//
//	func (b *Builder) addOTLPExporter() BuildComponentFunc[*LogPipeline] {
//	    return b.AddExporter(
//	        func(lp *LogPipeline) string { return fmt.Sprintf("otlp/%s", lp.Name) },
//	        func(ctx context.Context, lp *LogPipeline) (any, EnvVars, error) {
//	            builder := NewOTLPExporterConfigBuilder(
//	                b.Reader, lp.Spec.Output.OTLP, lp.Name, queueSize, SignalTypeLog,
//	            )
//	            return builder.OTLPExporter(ctx)
//	        },
//	    )
//	}
func (cb *ComponentBuilder[T]) AddExporter(componentIDFunc ComponentIDFunc[T], configFunc ExporterComponentConfigFunc[T]) BuildComponentFunc[T] {
	return func(ctx context.Context, pipeline T, pipelineID string) error {
		exporterConfig, exporterEnvVars, err := configFunc(ctx, pipeline)
		if err != nil {
			return fmt.Errorf("failed to create exporter config: %w", err)
		}

		if exporterConfig == nil {
			// If no config is provided, skip adding the exporter
			return nil
		}

		componentID := componentIDFunc(pipeline)

		exportersOrConnectors := cb.Collector.Exporters
		if isConnector(componentID) {
			exportersOrConnectors = cb.Collector.Connectors
		}

		exportersOrConnectors[componentID] = exporterConfig

		maps.Copy(cb.EnvVars, exporterEnvVars)

		servicePipeline := cb.Collector.Service.Pipelines[pipelineID]
		servicePipeline.Exporters = append(servicePipeline.Exporters, componentID)
		cb.Collector.Service.Pipelines[pipelineID] = servicePipeline

		return nil
	}
}

func (cb *ComponentBuilder[T]) AddExtension(componentID string, extensionConfig any, extensionEnvVars EnvVars) {
	if _, found := cb.Collector.Extensions[componentID]; !found {
		cb.Collector.Extensions[componentID] = extensionConfig
	}

	if extensionEnvVars != nil {
		maps.Copy(cb.EnvVars, extensionEnvVars)
	}

	// Ensure the extension is added to the service only once
	extensions := cb.Collector.Service.Extensions
	if slices.Contains(extensions, componentID) {
		return
	}

	cb.Collector.Service.Extensions = append(cb.Collector.Service.Extensions, componentID)
}

// StaticComponentID returns a ComponentIDFunc that always returns the same ID.
// Useful for static components like receivers and processors.
func (cb *ComponentBuilder[T]) StaticComponentID(componentID string) ComponentIDFunc[T] {
	return func(pipeline T) string {
		return componentID
	}
}

func isConnector(componentID string) bool {
	return strings.HasPrefix(componentID, "routing") || strings.HasPrefix(componentID, "forward")
}
