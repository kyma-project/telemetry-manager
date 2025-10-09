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

type buildComponentFunc = common.BuildComponentFunc[*telemetryv1alpha1.TracePipeline]

type Builder struct {
	common.ComponentBuilder[*telemetryv1alpha1.TracePipeline]

	Reader client.Reader
}

type BuildOptions struct {
	ClusterName   string
	ClusterUID    string
	CloudProvider string
	Enrichments   *operatorv1alpha1.EnrichmentSpec
}

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.TracePipeline, opts BuildOptions) (*common.Config, common.EnvVars, error) {
	b.Config = common.NewConfig()
	b.EnvVars = make(common.EnvVars)

	// Iterate over each TracePipeline CR and enrich the config with pipeline-specific components
	queueSize := common.BatchingMaxQueueSize / len(pipelines)

	for _, pipeline := range pipelines {
		pipelineID := formatTraceServicePipelineID(&pipeline)
		if err := b.AddServicePipeline(ctx, &pipeline, pipelineID,
			b.addOTLPReceiver(),
			b.addMemoryLimiterProcessor(),
			b.addK8sAttributesProcessor(opts),
			b.addIstioNoiseFilterProcessor(),
			b.addInsertClusterAttributesProcessor(opts),
			b.addServiceEnrichmentProcessor(),
			b.addDropKymaAttributesProcessor(),
			b.addUserDefinedTransformProcessor(),
			b.addUserDefinedFilterProcessor(),
			b.addBatchProcessor(),
			b.addOTLPExporter(queueSize),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add service pipeline: %w", err)
		}
	}

	return b.Config, b.EnvVars, nil
}

func (b *Builder) addOTLPReceiver() buildComponentFunc {
	return b.AddReceiver(
		b.StaticComponentID(common.ComponentIDOTLPReceiver),
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
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDMemoryLimiterProcessor),
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
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDK8sAttributesProcessor),
		func(tp *telemetryv1alpha1.TracePipeline) any {
			return common.K8sAttributesProcessorConfig(opts.Enrichments)
		},
	)
}

func (b *Builder) addIstioNoiseFilterProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDIstioNoiseFilterProcessor),
		func(tp *telemetryv1alpha1.TracePipeline) any {
			return &common.IstioNoiseFilterProcessor{}
		},
	)
}

func (b *Builder) addInsertClusterAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDInsertClusterAttributesProcessor),
		func(tp *telemetryv1alpha1.TracePipeline) any {
			return common.InsertClusterAttributesProcessorConfig(
				opts.ClusterName, opts.ClusterUID, opts.CloudProvider,
			)
		},
	)
}

func (b *Builder) addServiceEnrichmentProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDServiceEnrichmentProcessor),
		func(tp *telemetryv1alpha1.TracePipeline) any {
			return common.ResolveServiceNameConfig()
		},
	)
}

func (b *Builder) addDropKymaAttributesProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropKymaAttributesProcessor),
		func(tp *telemetryv1alpha1.TracePipeline) any {
			return common.DropKymaAttributesProcessorConfig()
		},
	)
}

// addUserDefinedTransformProcessor handles user-defined transform processors with dynamic component IDs
func (b *Builder) addUserDefinedTransformProcessor() buildComponentFunc {
	return b.AddProcessor(
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

func (b *Builder) addUserDefinedFilterProcessor() buildComponentFunc {
	return b.AddProcessor(
		formatUserDefinedFilterProcessorID,
		func(tp *telemetryv1alpha1.TracePipeline) any {
			if len(tp.Spec.Filter) == 0 {
				return nil
			}

			filterStatements := common.FilterSpecsToProcessorStatements(tp.Spec.Filter)
			filterProcessor := common.TraceFilterProcessorConfig(filterStatements)

			return filterProcessor
		})
}

//nolint:mnd // hardcoded values
func (b *Builder) addBatchProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDBatchProcessor),
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
	return b.AddExporter(
		formatOTLPExporterID,
		func(ctx context.Context, tp *telemetryv1alpha1.TracePipeline) (any, common.EnvVars, error) {
			otlpExporterBuilder := common.NewOTLPExporterConfigBuilder(
				b.Reader,
				tp.Spec.Output.OTLP,
				tp.Name,
				queueSize,
				common.SignalTypeTrace,
			)

			return otlpExporterBuilder.OTLPExporterConfig(ctx)
		},
	)
}

func formatTraceServicePipelineID(tp *telemetryv1alpha1.TracePipeline) string {
	return fmt.Sprintf("traces/%s", tp.Name)
}

func formatUserDefinedTransformProcessorID(tp *telemetryv1alpha1.TracePipeline) string {
	return fmt.Sprintf(common.ComponentIDUserDefinedTransformProcessor, tp.Name)
}

func formatUserDefinedFilterProcessorID(tp *telemetryv1alpha1.TracePipeline) string {
	return fmt.Sprintf(common.ComponentIDUserDefinedFilterProcessor, tp.Name)
}

func formatOTLPExporterID(tp *telemetryv1alpha1.TracePipeline) string {
	return common.ExporterID(tp.Spec.Output.OTLP.Protocol, tp.Name)
}
