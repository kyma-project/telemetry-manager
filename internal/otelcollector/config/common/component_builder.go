package common

import (
	"context"
	"fmt"
	"maps"
)

// BuildComponentFunc defines a function type for building OpenTelemetry collector components.
// It can be chained together to construct telemetry pipelines.
//
// Example:
//
//	err := b.addServicePipeline(ctx, &pipeline,
//	    b.addOTLPReceiver(),
//	    b.addMemoryLimiterProcessor(),
//	    b.addOTLPExporter(),
//	)
type BuildComponentFunc[T any] func(ctx context.Context, pipeline T) error

// ComponentIDFunc determines the unique identifier for a component.
//
// Example:
//
//	staticID := StaticComponentID[*LogPipeline]("memory_limiter")
//	dynamicID := func(lp *LogPipeline) string {
//	    return fmt.Sprintf("otlp/%s", lp.Name)
//	}
type ComponentIDFunc[T any] func(pipeline T) string

// ComponentConfigFunc creates the configuration for a component (receiver or processor).
// Returns nil to skip the component for this pipeline.
//
// Example:
//
//	func otlpReceiverConfig(lp *LogPipeline) any {
//	    return &OTLPReceiver{
//	        Protocols: ReceiverProtocols{
//	            HTTP: Endpoint{Endpoint: "0.0.0.0:4318"},
//	        },
//	    }
//	}
type ComponentConfigFunc[T any] func(pipeline T) any

// ExporterComponentConfigFunc creates exporter configuration and environment variables.
// Unlike receivers/processors, exporters often need secret resolution.
//
// Example:
//
//	func otlpExporterConfig(ctx context.Context, lp *LogPipeline) (any, EnvVars, error) {
//	    envVars := make(EnvVars)
//	    envVars["OTLP_ENDPOINT"] = []byte("https://backend.example.com")
//	    config := &OTLPExporter{Endpoint: "${OTLP_ENDPOINT}"}
//	    return config, envVars, nil
//	}
type ExporterComponentConfigFunc[T any] func(ctx context.Context, pipeline T) (any, EnvVars, error)

// PipelineIDFunc determines the unique identifier for a service pipeline.
//
// Example:
//
//	func formatLogServicePipelineID(lp *LogPipeline) string {
//	    return fmt.Sprintf("logs/%s", lp.Name)
//	}
type PipelineIDFunc[T any] func(pipeline T) string

// StaticComponentID returns a ComponentIDFunc that always returns the same ID.
// Useful for shared components like receivers and processors.
//
// Example:
//
//	otlpReceiverID := StaticComponentID[*LogPipeline]("otlp")
//	// Results in: receivers: otlp: {...}
func StaticComponentID[T any](componentID string) ComponentIDFunc[T] {
	return func(T) string {
		return componentID
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
//	            return &OTLPReceiver{...}
//	        },
//	        formatLogServicePipelineID,
//	    )
//	}
func AddReceiver[T any](
	rootConfig *Config,
	componentIDFunc ComponentIDFunc[T],
	configFunc ComponentConfigFunc[T],
	pipelineIDFunc PipelineIDFunc[T],
) BuildComponentFunc[T] {
	return func(ctx context.Context, pipeline T) error {
		receiverConfig := configFunc(pipeline)
		if receiverConfig == nil {
			// If no config is provided, skip adding the receiver
			return nil
		}

		componentID := componentIDFunc(pipeline)
		if _, found := rootConfig.Receivers[componentID]; !found {
			rootConfig.Receivers[componentID] = receiverConfig
		}

		pipelineID := pipelineIDFunc(pipeline)
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
//	            return &MemoryLimiter{...}
//	        },
//	        formatLogServicePipelineID,
//	    )
//	}
func AddProcessor[T any](
	rootConfig *Config,
	componentIDFunc ComponentIDFunc[T],
	configFunc ComponentConfigFunc[T],
	pipelineIDFunc PipelineIDFunc[T],
) BuildComponentFunc[T] {
	return func(ctx context.Context, pipeline T) error {
		processorConfig := configFunc(pipeline)
		if processorConfig == nil {
			// If no config is provided, skip adding the processor
			return nil
		}

		componentID := componentIDFunc(pipeline)
		if _, found := rootConfig.Processors[componentID]; !found {
			rootConfig.Processors[componentID] = processorConfig
		}

		pipelineID := pipelineIDFunc(pipeline)
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
//	            // Create config and env vars
//	            return config, envVars, nil
//	        },
//	        formatLogServicePipelineID,
//	    )
//	}
func AddExporter[T any](
	rootConfig *Config,
	envVars EnvVars,
	componentIDFunc ComponentIDFunc[T],
	configFunc ExporterComponentConfigFunc[T],
	pipelineIDFunc PipelineIDFunc[T],
) BuildComponentFunc[T] {
	return func(ctx context.Context, pipeline T) error {
		exporterConfig, exporterEnvVars, err := configFunc(ctx, pipeline)
		if err != nil {
			return fmt.Errorf("failed to create exporter config: %w", err)
		}

		if exporterConfig == nil {
			// If no config is provided, skip adding the exporter
			return nil
		}

		componentID := componentIDFunc(pipeline)
		rootConfig.Exporters[componentID] = exporterConfig

		maps.Copy(envVars, exporterEnvVars)

		pipelineID := pipelineIDFunc(pipeline)
		servicePipeline := rootConfig.Service.Pipelines[pipelineID]
		servicePipeline.Exporters = append(servicePipeline.Exporters, componentID)
		rootConfig.Service.Pipelines[pipelineID] = servicePipeline

		return nil
	}
}
