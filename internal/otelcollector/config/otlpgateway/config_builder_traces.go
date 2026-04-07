package otlpgateway

import (
	"context"
	"fmt"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
)

// buildTracePipelines builds trace pipeline configuration and adds it to the shared config.
func (b *Builder) buildTracePipelines(ctx context.Context, builder *common.ComponentBuilder[*telemetryv1beta1.TracePipeline], opts BuildOptions) error {
	pipelines := opts.TracePipelines
	if len(pipelines) == 0 {
		return nil
	}

	queueSize := common.BatchingMaxQueueSize / len(pipelines)

	for _, pipeline := range pipelines {
		pipelineID := formatTraceServicePipelineID(&pipeline)

		if shouldEnableTraceOAuth2(&pipeline) {
			if err := b.addTraceOAuth2Extension(ctx, builder, &pipeline); err != nil {
				return err
			}
		}

		if err := builder.AddServicePipeline(ctx, &pipeline, pipelineID,
			b.addTraceOTLPReceiver(builder),
			b.addTraceMemoryLimiterProcessor(builder),
			b.addDropIstioServiceEnrichmentProcessor(builder, opts),
			b.addTraceDropUnknownServiceNameProcessor(builder, opts),
			b.addTraceK8sAttributesProcessor(builder, opts),
			b.addTraceIstioNoiseFilterProcessor(builder),
			b.addTraceInsertClusterAttributesProcessor(builder, opts),
			b.addTraceServiceEnrichmentProcessor(builder, opts),
			b.addTraceDropKymaAttributesProcessor(builder),
			b.addTraceUserDefinedTransformProcessor(builder),
			b.addTraceUserDefinedFilterProcessor(builder),
			b.addTraceBatchProcessor(builder),
			b.addTraceOTLPExporter(builder, queueSize),
		); err != nil {
			return fmt.Errorf("failed to add trace service pipeline: %w", err)
		}
	}

	return nil
}

func (b *Builder) addTraceOTLPReceiver(builder *common.ComponentBuilder[*telemetryv1beta1.TracePipeline]) buildTraceComponentFunc {
	return builder.AddReceiver(
		builder.StaticComponentID(common.ComponentIDOTLPReceiver),
		func(tp *telemetryv1beta1.TracePipeline) any {
			return &common.OTLPReceiverConfig{
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
func (b *Builder) addTraceMemoryLimiterProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.TracePipeline]) buildTraceComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDMemoryLimiterProcessor),
		func(tp *telemetryv1beta1.TracePipeline) any {
			return &common.MemoryLimiterConfig{
				CheckInterval:        "1s",
				LimitPercentage:      75,
				SpikeLimitPercentage: 15,
			}
		},
	)
}

func (b *Builder) addDropIstioServiceEnrichmentProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.TracePipeline], opts BuildOptions) buildTraceComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDDropIstioServiceEnrichmentProcessor),
		func(tp *telemetryv1beta1.TracePipeline) any {
			if opts.ServiceEnrichment != commonresources.AnnotationValueTelemetryServiceEnrichmentOtel {
				return nil
			}

			transformStatements := []common.TransformProcessorStatements{{
				Statements: []string{
					"delete_matching_keys(resource.attributes, \"service.*\") where span.attributes[\"component\"] == \"proxy\"",
				},
			}}

			return common.TraceTransformProcessor(transformStatements)
		},
	)
}

func (b *Builder) addTraceDropUnknownServiceNameProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.TracePipeline], opts BuildOptions) buildTraceComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDDropUnknownServiceNameProcessor),
		func(tp *telemetryv1beta1.TracePipeline) any {
			if opts.ServiceEnrichment != commonresources.AnnotationValueTelemetryServiceEnrichmentOtel {
				return nil
			}

			return common.TraceTransformProcessor(common.DropUnknownServiceNameProcessorStatements())
		},
	)
}

func (b *Builder) addTraceK8sAttributesProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.TracePipeline], opts BuildOptions) buildTraceComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDK8sAttributesProcessor),
		func(tp *telemetryv1beta1.TracePipeline) any {
			useOTelServiceEnrichment := opts.ServiceEnrichment == commonresources.AnnotationValueTelemetryServiceEnrichmentOtel
			return common.K8sAttributesProcessor(opts.Enrichments, useOTelServiceEnrichment)
		},
	)
}

func (b *Builder) addTraceIstioNoiseFilterProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.TracePipeline]) buildTraceComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDIstioNoiseFilterProcessor),
		func(tp *telemetryv1beta1.TracePipeline) any {
			return &common.IstioNoiseFilterProcessorConfig{}
		},
	)
}

func (b *Builder) addTraceInsertClusterAttributesProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.TracePipeline], opts BuildOptions) buildTraceComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDInsertClusterAttributesProcessor),
		func(tp *telemetryv1beta1.TracePipeline) any {
			transformStatements := common.InsertClusterAttributesProcessorStatements(opts.Cluster)
			return common.TraceTransformProcessor(transformStatements)
		},
	)
}

func (b *Builder) addTraceServiceEnrichmentProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.TracePipeline], opts BuildOptions) buildTraceComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDServiceEnrichmentProcessor),
		func(tp *telemetryv1beta1.TracePipeline) any {
			if opts.ServiceEnrichment == commonresources.AnnotationValueTelemetryServiceEnrichmentOtel {
				return nil
			}

			return common.ResolveServiceName()
		},
	)
}

func (b *Builder) addTraceDropKymaAttributesProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.TracePipeline]) buildTraceComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDDropKymaAttributesProcessor),
		func(tp *telemetryv1beta1.TracePipeline) any {
			transformStatements := common.DropKymaAttributesProcessorStatements()
			return common.TraceTransformProcessor(transformStatements)
		},
	)
}

func (b *Builder) addTraceUserDefinedTransformProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.TracePipeline]) buildTraceComponentFunc {
	return builder.AddProcessor(
		formatTraceUserDefinedTransformProcessorID,
		func(tp *telemetryv1beta1.TracePipeline) any {
			if len(tp.Spec.Transforms) == 0 {
				return nil
			}

			transformStatements := common.TransformSpecsToProcessorStatements(tp.Spec.Transforms)

			return common.TraceTransformProcessor(transformStatements)
		},
	)
}

func (b *Builder) addTraceUserDefinedFilterProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.TracePipeline]) buildTraceComponentFunc {
	return builder.AddProcessor(
		formatTraceUserDefinedFilterProcessorID,
		func(tp *telemetryv1beta1.TracePipeline) any {
			if tp.Spec.Filters == nil {
				return nil
			}

			return common.TraceFilterProcessor(tp.Spec.Filters)
		},
	)
}

//nolint:mnd // magic numbers for batch processor configuration
func (b *Builder) addTraceBatchProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.TracePipeline]) buildTraceComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDBatchProcessor),
		func(_ *telemetryv1beta1.TracePipeline) any {
			return &common.BatchProcessorConfig{
				SendBatchSize:    512,
				Timeout:          "10s",
				SendBatchMaxSize: 512,
			}
		},
	)
}

func (b *Builder) addTraceOTLPExporter(builder *common.ComponentBuilder[*telemetryv1beta1.TracePipeline], queueSize int) buildTraceComponentFunc {
	return builder.AddExporter(
		formatTraceOTLPExporterID,
		func(ctx context.Context, tp *telemetryv1beta1.TracePipeline) (any, common.EnvVars, error) {
			otlpExporterBuilder := common.NewOTLPExporterConfigBuilder(
				b.Reader,
				tp.Spec.Output.OTLP,
				common.TracePipelineRef(tp),
				queueSize,
			)

			return otlpExporterBuilder.OTLPExporter(ctx)
		},
	)
}

//nolint:dupl // Acceptable duplication - trace and log OAuth2 extensions follow same pattern
func (b *Builder) addTraceOAuth2Extension(ctx context.Context, builder *common.ComponentBuilder[*telemetryv1beta1.TracePipeline], pipeline *telemetryv1beta1.TracePipeline) error {
	pipelineRef := common.TracePipelineRef(pipeline)
	oauth2ExtensionID := common.OAuth2ExtensionID(pipelineRef)

	oauth2ExtensionConfig, oauth2ExtensionEnvVars, err := common.NewOAuth2ExtensionConfigBuilder(
		b.Reader,
		pipeline.Spec.Output.OTLP.Authentication.OAuth2,
		pipelineRef,
	).OAuth2Extension(ctx)
	if err != nil {
		return fmt.Errorf("failed to build OAuth2 extension for pipeline %s: %w", pipeline.Name, err)
	}

	builder.AddExtension(oauth2ExtensionID, oauth2ExtensionConfig, oauth2ExtensionEnvVars)

	return nil
}

// Trace pipeline helper functions

func shouldEnableTraceOAuth2(tp *telemetryv1beta1.TracePipeline) bool {
	return tp.Spec.Output.OTLP.Authentication != nil && tp.Spec.Output.OTLP.Authentication.OAuth2 != nil
}

func formatTraceServicePipelineID(tp *telemetryv1beta1.TracePipeline) string {
	return fmt.Sprintf("traces/%s", tp.Name)
}

func formatTraceUserDefinedTransformProcessorID(tp *telemetryv1beta1.TracePipeline) string {
	ref := common.TracePipelineRef(tp)
	return fmt.Sprintf(common.ComponentIDUserDefinedTransformProcessor, ref.TypePrefix(), ref.Name())
}

func formatTraceUserDefinedFilterProcessorID(tp *telemetryv1beta1.TracePipeline) string {
	ref := common.TracePipelineRef(tp)
	return fmt.Sprintf(common.ComponentIDUserDefinedFilterProcessor, ref.TypePrefix(), ref.Name())
}

func formatTraceOTLPExporterID(tp *telemetryv1beta1.TracePipeline) string {
	return common.ExporterID(tp.Spec.Output.OTLP.Protocol, common.TracePipelineRef(tp))
}
