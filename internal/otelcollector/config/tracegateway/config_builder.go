package tracegateway

import (
	"context"
	"fmt"

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

	config  *common.Config
	envVars common.EnvVars
}

type BuildOptions struct {
	ClusterName   string
	ClusterUID    string
	CloudProvider string
	Enrichments   *operatorv1alpha1.EnrichmentSpec
}

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.TracePipeline, opts BuildOptions) (*common.Config, common.EnvVars, error) {
	b.config = &common.Config{
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

// Type aliases for common builder patterns
type buildComponentFunc = common.BuildComponentFunc[*telemetryv1alpha1.TracePipeline]
type componentConfigFunc = common.ComponentConfigFunc[*telemetryv1alpha1.TracePipeline]
type exporterComponentConfigFunc = common.ExporterComponentConfigFunc[*telemetryv1alpha1.TracePipeline]
type componentIDFunc = common.ComponentIDFunc[*telemetryv1alpha1.TracePipeline]

// staticComponentID returns a ComponentIDFunc that always returns the same component ID independent of the TracePipeline
var staticComponentID = common.StaticComponentID[*telemetryv1alpha1.TracePipeline]

func (b *Builder) addServicePipeline(ctx context.Context, tp *telemetryv1alpha1.TracePipeline, fs ...buildComponentFunc) error {
	// Add an empty pipeline to the config
	pipelineID := formatTraceServicePipelineID(tp)
	b.config.Service.Pipelines[pipelineID] = common.Pipeline{}

	for _, f := range fs {
		if err := f(ctx, tp); err != nil {
			return fmt.Errorf("failed to add component: %w", err)
		}
	}

	return nil
}

// addReceiver creates a decorator for adding receivers
func (b *Builder) addReceiver(componentIDFunc componentIDFunc, configFunc componentConfigFunc) buildComponentFunc {
	return common.AddReceiver(
		b.config,
		componentIDFunc,
		configFunc,
		formatTraceServicePipelineID,
	)
}

// addProcessor creates a decorator for adding processors
func (b *Builder) addProcessor(componentIDFunc componentIDFunc, configFunc componentConfigFunc) buildComponentFunc {
	return common.AddProcessor(
		b.config,
		componentIDFunc,
		configFunc,
		formatTraceServicePipelineID,
	)
}

// addExporter creates a decorator for adding exporters
func (b *Builder) addExporter(componentIDFunc componentIDFunc, configFunc exporterComponentConfigFunc) buildComponentFunc {
	return common.AddExporter(
		b.config,
		b.envVars,
		componentIDFunc,
		configFunc,
		formatTraceServicePipelineID,
	)
}

func (b *Builder) addOTLPReceiver() buildComponentFunc {
	return b.addReceiver(
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
	return b.addProcessor(
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
	return b.addProcessor(
		staticComponentID(common.ComponentIDK8sAttributesProcessor),
		func(tp *telemetryv1alpha1.TracePipeline) any {
			return common.K8sAttributesProcessorConfig(opts.Enrichments)
		},
	)
}

func (b *Builder) addIstioNoiseFilterProcessor() buildComponentFunc {
	return b.addProcessor(
		staticComponentID(common.ComponentIDIstioNoiseFilterProcessor),
		func(tp *telemetryv1alpha1.TracePipeline) any {
			return &common.IstioNoiseFilterProcessor{}
		},
	)
}

func (b *Builder) addInsertClusterAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.addProcessor(
		staticComponentID(common.ComponentIDInsertClusterAttributesProcessor),
		func(tp *telemetryv1alpha1.TracePipeline) any {
			return common.InsertClusterAttributesProcessorConfig(
				opts.ClusterName, opts.ClusterUID, opts.CloudProvider,
			)
		},
	)
}

func (b *Builder) addServiceEnrichmentProcessor() buildComponentFunc {
	return b.addProcessor(
		staticComponentID(common.ComponentIDServiceEnrichmentProcessor),
		func(tp *telemetryv1alpha1.TracePipeline) any {
			return common.ResolveServiceNameConfig()
		},
	)
}

func (b *Builder) addDropKymaAttributesProcessor() buildComponentFunc {
	return b.addProcessor(
		staticComponentID(common.ComponentIDDropKymaAttributesProcessor),
		func(tp *telemetryv1alpha1.TracePipeline) any {
			return common.DropKymaAttributesProcessorConfig()
		},
	)
}

// addUserDefinedTransformProcessor handles user-defined transform processors with dynamic component IDs
func (b *Builder) addUserDefinedTransformProcessor() buildComponentFunc {
	return b.addProcessor(
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
	return b.addProcessor(
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
	return b.addExporter(
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
	return common.FormatServicePipelineID("traces", tp.Name)
}

func formatUserDefinedTransformProcessorID(tp *telemetryv1alpha1.TracePipeline) string {
	return common.FormatUserDefinedTransformProcessorID(tp.Name)
}

func formatOTLPExporterID(tp *telemetryv1alpha1.TracePipeline) string {
	return common.FormatExporterID(tp.Spec.Output.OTLP.Protocol, tp.Name)
}
