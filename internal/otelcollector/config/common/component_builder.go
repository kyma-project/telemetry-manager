package common

import (
	"context"
	"fmt"
	"maps"
	"strings"
)

// BuildComponentFunc defines a function type for building OpenTelemetry collector components.
// It can be chained together to construct telemetry pipelines.
type BuildComponentFunc[T any] func(ctx context.Context, buildCtx T, pipelineID string) error

// ComponentIDFunc determines the unique identifier for a component.
type ComponentIDFunc[T any] func(buildCtx T) string

// ComponentConfigFunc creates the configuration for a component (receiver or processor).
// Returns nil to skip the component for this pipeline.
type ComponentConfigFunc[T any] func(buildCtx T) any

// ExporterComponentConfigFunc creates exporter configuration and environment variables.
// Unlike receivers/processors, exporters often need secret resolution.
type ExporterComponentConfigFunc[T any] func(ctx context.Context, buildCtx T) (any, EnvVars, error)

// PipelineIDFunc determines the unique identifier for a service pipeline.
type PipelineIDFunc[T any] func(buildCtx T) string

// StaticComponentID returns a ComponentIDFunc that always returns the same ID.
// Useful for shared components like receivers and processors.
func StaticComponentID[T any](componentID string) ComponentIDFunc[T] {
	return func(T) string {
		return componentID
	}
}

// ComponentBuilder provides common builder patterns for OpenTelemetry collector configurations.
// It can be embedded into specific config builders to provide reusable component management.
type ComponentBuilder[T any] struct {
	Config  *Config
	EnvVars EnvVars
}

func (cb *ComponentBuilder[T]) AddServicePipeline(ctx context.Context, buildCtx T, pipelineID string, fs ...BuildComponentFunc[T]) error {
	// Initialize pipeline componentsAdd an empty pipeline to the config
	cb.Config.Service.Pipelines[pipelineID] = Pipeline{}

	for _, f := range fs {
		// None of the service pipelines depend on the MetricPipeline object, so we can pass nil here
		if err := f(ctx, buildCtx, pipelineID); err != nil {
			return fmt.Errorf("failed to add component: %w", err)
		}
	}

	return nil
}

// AddReceiver creates a BuildComponentWithIDFunc that adds a receiver to the configuration using a pipeline ID.
func (cb *ComponentBuilder[T]) AddReceiver(componentIDFunc ComponentIDFunc[T], configFunc ComponentConfigFunc[T]) BuildComponentFunc[T] {
	return func(ctx context.Context, buildCtx T, pipelineID string) error {
		return AddReceiver(cb.Config, componentIDFunc, configFunc)(ctx, buildCtx, pipelineID)
	}
}

// AddProcessor creates a BuildComponentWithIDFunc that adds a processor to the configuration using a pipeline ID.
func (cb *ComponentBuilder[T]) AddProcessor(componentIDFunc ComponentIDFunc[T], configFunc ComponentConfigFunc[T]) BuildComponentFunc[T] {
	return func(ctx context.Context, buildCtx T, pipelineID string) error {
		return AddProcessor(cb.Config, componentIDFunc, configFunc)(ctx, buildCtx, pipelineID)
	}
}

// AddExporter creates a BuildComponentWithIDFunc that adds an exporter to the configuration using a pipeline ID.
func (cb *ComponentBuilder[T]) AddExporter(componentIDFunc ComponentIDFunc[T], configFunc ExporterComponentConfigFunc[T]) BuildComponentFunc[T] {
	return func(ctx context.Context, buildCtx T, pipelineID string) error {
		return AddExporter(cb.Config, cb.EnvVars, componentIDFunc, configFunc)(ctx, buildCtx, pipelineID)
	}
}

// AddReceiver creates a BuildComponentFunc that adds a receiver to the configuration.
// Receivers collect telemetry data from various sources.
//
// Example:
//
//	func (b *Builder) addOTLPReceiver() BuildComponentFunc[*LogPipeline] {
//	    return AddReceiver(
//	        b.config,
//	        StaticComponentID[*LogPipeline]("otlp"),
//	        func(lp *LogPipeline) any {
//	            return &OTLPReceiver{
//	                Protocols: ReceiverProtocols{
//	                    HTTP: Endpoint{Endpoint: fmt.Sprintf("${%s}:4318", EnvVarCurrentPodIP)},
//	                    GRPC: Endpoint{Endpoint: fmt.Sprintf("${%s}:4317", EnvVarCurrentPodIP)},
//	                },
//	            }
//	        },
//	        func(lp *LogPipeline) string { return fmt.Sprintf("logs/%s", lp.Name) },
//	    )
//	}
//
//nolint:dupl // Code duplication is intentional for clarity
func AddReceiver[T any](
	rootConfig *Config,
	componentIDFunc ComponentIDFunc[T],
	configFunc ComponentConfigFunc[T],
) BuildComponentFunc[T] {
	return func(ctx context.Context, buildCtx T, pipelineID string) error {
		receiverConfig := configFunc(buildCtx)
		if receiverConfig == nil {
			// If no config is provided, skip adding the receiver
			return nil
		}

		componentID := componentIDFunc(buildCtx)

		receiversOrConnectors := rootConfig.Receivers
		if isConnector(componentID) {
			receiversOrConnectors = rootConfig.Connectors
		}

		if _, found := receiversOrConnectors[componentID]; !found {
			receiversOrConnectors[componentID] = receiverConfig
		}

		pipelineConfig := rootConfig.Service.Pipelines[pipelineID]
		pipelineConfig.Receivers = append(pipelineConfig.Receivers, componentID)
		rootConfig.Service.Pipelines[pipelineID] = pipelineConfig

		return nil
	}
}

// AddProcessor creates a BuildComponentFunc that adds a processor to the configuration.
// Processors transform, filter, or enrich telemetry data. Order matters in pipelines.
//
// Example:
//
//	func (b *Builder) addMemoryLimiterProcessor() BuildComponentFunc[*LogPipeline] {
//	    return AddProcessor(
//	        b.config,
//	        StaticComponentID[*LogPipeline]("memory_limiter"),
//	        func(lp *LogPipeline) any {
//	            return &MemoryLimiter{
//	                CheckInterval:        "1s",
//	                LimitPercentage:      75,
//	                SpikeLimitPercentage: 15,
//	            }
//	        },
//	        func(lp *LogPipeline) string { return fmt.Sprintf("logs/%s", lp.Name) },
//	    )
//	}
//
//nolint:dupl // Code duplication is intentional for clarity
func AddProcessor[T any](
	rootConfig *Config,
	componentIDFunc ComponentIDFunc[T],
	configFunc ComponentConfigFunc[T],
) BuildComponentFunc[T] {
	return func(ctx context.Context, pipeline T, pipelineID string) error {
		processorConfig := configFunc(pipeline)
		if processorConfig == nil {
			// If no config is provided, skip adding the processor
			return nil
		}

		componentID := componentIDFunc(pipeline)
		if _, found := rootConfig.Processors[componentID]; !found {
			rootConfig.Processors[componentID] = processorConfig
		}

		servicePipeline := rootConfig.Service.Pipelines[pipelineID]
		servicePipeline.Processors = append(servicePipeline.Processors, componentID)
		rootConfig.Service.Pipelines[pipelineID] = servicePipeline

		return nil
	}
}

// AddExporter creates a BuildComponentFunc that adds an exporter to the configuration.
// Exporters send telemetry data to external systems and often require secret resolution.
//
// Example:
//
//	func (b *Builder) addOTLPExporter() BuildComponentFunc[*LogPipeline] {
//	    return AddExporter(
//	        b.config,
//	        b.envVars,
//	        func(lp *LogPipeline) string { return fmt.Sprintf("otlp/%s", lp.Name) },
//	        func(ctx context.Context, lp *LogPipeline) (any, EnvVars, error) {
//	            builder := NewOTLPExporterConfigBuilder(
//	                b.Reader, lp.Spec.Output.OTLP, lp.Name, queueSize, SignalTypeLog,
//	            )
//	            return builder.OTLPExporterConfig(ctx)
//	        },
//	        func(lp *LogPipeline) string { return fmt.Sprintf("logs/%s", lp.Name) },
//	    )
//	}
func AddExporter[T any](
	rootConfig *Config,
	envVars EnvVars,
	componentIDFunc ComponentIDFunc[T],
	configFunc ExporterComponentConfigFunc[T],
) BuildComponentFunc[T] {
	return func(ctx context.Context, buildCtx T, pipelineID string) error {
		exporterConfig, exporterEnvVars, err := configFunc(ctx, buildCtx)
		if err != nil {
			return fmt.Errorf("failed to create exporter config: %w", err)
		}

		if exporterConfig == nil {
			// If no config is provided, skip adding the exporter
			return nil
		}

		componentID := componentIDFunc(buildCtx)

		exportersOrConnectors := rootConfig.Exporters
		if isConnector(componentID) {
			exportersOrConnectors = rootConfig.Connectors
		}

		exportersOrConnectors[componentID] = exporterConfig

		maps.Copy(envVars, exporterEnvVars)

		servicePipeline := rootConfig.Service.Pipelines[pipelineID]
		servicePipeline.Exporters = append(servicePipeline.Exporters, componentID)
		rootConfig.Service.Pipelines[pipelineID] = servicePipeline

		return nil
	}
}

func isConnector(componentID string) bool {
	return strings.HasPrefix(componentID, "routing") || strings.HasPrefix(componentID, "forward")
}
