package common

import (
	"context"
	"fmt"
	"maps"
)

// BuildComponentFunc defines a function type for building components in the telemetry configuration
type BuildComponentFunc[T any] func(ctx context.Context, pipeline T) error

// ComponentConfigFunc creates the configuration for a component (receiver or processor)
type ComponentConfigFunc[T any] func(pipeline T) any

// ExporterComponentConfigFunc creates the configuration for an exporter component
// creating exporters is different from receivers and processors, as it makes an API server call to resolve the reference secrets
// and returns the configuration along with environment variables needed for the exporter
type ExporterComponentConfigFunc[T any] func(ctx context.Context, pipeline T) (any, EnvVars, error)

// ComponentIDFunc determines the ID of a component
type ComponentIDFunc[T any] func(pipeline T) string

// StaticComponentID returns a ComponentIDFunc that always returns the same component ID independent of the pipeline
func StaticComponentID[T any](componentID string) ComponentIDFunc[T] {
	return func(T) string {
		return componentID
	}
}

// FormatUserDefinedTransformProcessorID formats the ID for user-defined transform processors
func FormatUserDefinedTransformProcessorID(pipelineName string) string {
	return fmt.Sprintf("transform/user-defined-%s", pipelineName)
}

// FormatServicePipelineID formats the ID for service pipelines
func FormatServicePipelineID(signalType, pipelineName string) string {
	return fmt.Sprintf("%s/%s", signalType, pipelineName)
}

// AddReceiver creates a decorator for adding receivers
func AddReceiver[T any](
	rootConfig *Config,
	componentIDFunc ComponentIDFunc[T],
	configFunc ComponentConfigFunc[T],
	pipelineIDFunc func(T) string,
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

// AddProcessor creates a decorator for adding processors
func AddProcessor[T any](
	rootConfig *Config,
	componentIDFunc ComponentIDFunc[T],
	configFunc ComponentConfigFunc[T],
	pipelineIDFunc func(T) string,
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

// AddExporter creates a decorator for adding exporters
func AddExporter[T any](
	rootConfig *Config,
	envVars EnvVars,
	componentIDFunc ComponentIDFunc[T],
	configFunc ExporterComponentConfigFunc[T],
	pipelineIDFunc func(T) string,
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
