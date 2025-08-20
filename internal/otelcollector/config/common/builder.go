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

// AddServicePipeline adds a service pipeline by applying a series of component functions
func AddServicePipeline[T any](
	ctx context.Context,
	pipeline T,
	pipelineID string,
	initPipelineFunc func(string),
	fs ...BuildComponentFunc[T],
) error {
	// Initialize an empty pipeline
	initPipelineFunc(pipelineID)

	// Apply all component functions
	for _, f := range fs {
		if err := f(ctx, pipeline); err != nil {
			return fmt.Errorf("failed to add component: %w", err)
		}
	}

	return nil
}

// WithReceiver creates a decorator for adding receivers
func WithReceiver[T any, Config any](
	componentIDFunc ComponentIDFunc[T],
	configFunc ComponentConfigFunc[T],
	receivers map[string]any,
	pipelines map[string]Pipeline,
	pipelineIDFunc func(T) string,
) BuildComponentFunc[T] {
	return func(ctx context.Context, pipeline T) error {
		config := configFunc(pipeline)
		if config == nil {
			// If no config is provided, skip adding the receiver
			return nil
		}

		componentID := componentIDFunc(pipeline)
		if _, found := receivers[componentID]; !found {
			receivers[componentID] = config
		}

		pipelineID := pipelineIDFunc(pipeline)
		pipelineConfig := pipelines[pipelineID]
		pipelineConfig.Receivers = append(pipelineConfig.Receivers, componentID)
		pipelines[pipelineID] = pipelineConfig

		return nil
	}
}

// WithProcessor creates a decorator for adding processors
func WithProcessor[T any, Config any](
	componentIDFunc ComponentIDFunc[T],
	configFunc ComponentConfigFunc[T],
	processors map[string]any,
	pipelines map[string]Pipeline,
	pipelineIDFunc func(T) string,
) BuildComponentFunc[T] {
	return func(ctx context.Context, pipeline T) error {
		config := configFunc(pipeline)
		if config == nil {
			// If no config is provided, skip adding the processor
			return nil
		}

		componentID := componentIDFunc(pipeline)
		if _, found := processors[componentID]; !found {
			processors[componentID] = config
		}

		pipelineID := pipelineIDFunc(pipeline)
		pipelineConfig := pipelines[pipelineID]
		pipelineConfig.Processors = append(pipelineConfig.Processors, componentID)
		pipelines[pipelineID] = pipelineConfig

		return nil
	}
}

// WithExporter creates a decorator for adding exporters
func WithExporter[T any, Config any](
	componentIDFunc ComponentIDFunc[T],
	configFunc ExporterComponentConfigFunc[T],
	exporters map[string]any,
	pipelines map[string]Pipeline,
	envVars EnvVars,
	pipelineIDFunc func(T) string,
) BuildComponentFunc[T] {
	return func(ctx context.Context, pipeline T) error {
		config, exporterEnvVars, err := configFunc(ctx, pipeline)
		if err != nil {
			return fmt.Errorf("failed to create exporter config: %w", err)
		}

		if config == nil {
			// If no config is provided, skip adding the exporter
			return nil
		}

		componentID := componentIDFunc(pipeline)
		exporters[componentID] = config

		maps.Copy(envVars, exporterEnvVars)

		pipelineID := pipelineIDFunc(pipeline)
		pipelineConfig := pipelines[pipelineID]
		pipelineConfig.Exporters = append(pipelineConfig.Exporters, componentID)
		pipelines[pipelineID] = pipelineConfig

		return nil
	}
}
