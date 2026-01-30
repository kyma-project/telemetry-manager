package tracegateway

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
)

type buildComponentFunc = common.BuildComponentFunc[*telemetryv1beta1.TracePipeline]

type Builder struct {
	common.ComponentBuilder[*telemetryv1beta1.TracePipeline]

	Reader client.Reader
}

type BuildOptions struct {
	Cluster     common.ClusterOptions
	Enrichments *operatorv1beta1.EnrichmentSpec
	// ServiceEnrichment specifies the service enrichment strategy to be used (temporary)
	ServiceEnrichment string
}

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1beta1.TracePipeline, opts BuildOptions) (*common.Config, common.EnvVars, error) {
	b.Config = common.NewConfig()
	b.EnvVars = make(common.EnvVars)

	// Iterate over each TracePipeline CR and enrich the config with pipeline-specific components
	queueSize := common.BatchingMaxQueueSize / len(pipelines)

	for _, pipeline := range pipelines {
		pipelineID := formatTraceServicePipelineID(&pipeline)

		if shouldEnableOAuth2(&pipeline) {
			if err := b.addOAuth2Extension(ctx, &pipeline); err != nil {
				return nil, nil, err
			}
		}

		if err := b.AddServicePipeline(ctx, &pipeline, pipelineID,
			b.addOTLPReceiver(),
			b.addMemoryLimiterProcessor(),
			b.addDropIstioServiceEnrichmentProcessor(opts),
			b.addDropUnknownServiceNameProcessor(opts),
			b.addK8sAttributesProcessor(opts),
			b.addIstioNoiseFilterProcessor(),
			b.addInsertClusterAttributesProcessor(opts),
			b.addServiceEnrichmentProcessor(opts),
			// Kyma attributes are dropped before user-defined transform and filter processors
			// to prevent user access to internal attributes.
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
		func(tp *telemetryv1beta1.TracePipeline) any {
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
		func(tp *telemetryv1beta1.TracePipeline) any {
			return &common.MemoryLimiter{
				CheckInterval:        "1s",
				LimitPercentage:      75,
				SpikeLimitPercentage: 15,
			}
		},
	)
}

func (b *Builder) addDropIstioServiceEnrichmentProcessor(opts BuildOptions) buildComponentFunc {
	// Add the processor if OTel service enrichment strategy is selected
	// (temporary measure for dropping Istio trace spans enrichment)
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropIstioServiceEnrichmentProcessor),
		func(tp *telemetryv1beta1.TracePipeline) any {
			if opts.ServiceEnrichment != commonresources.AnnotationValueTelemetryServiceEnrichmentOtel {
				return nil // Kyma legacy enrichment selected, skip this processor
			}

			transformStatements := []common.TransformProcessorStatements{{
				Statements: []string{
					"delete_matching_keys(resource.attributes, \"service.*\") where span.attributes[\"component\"] == \"proxy\"",
				},
			}}

			return common.TraceTransformProcessorConfig(transformStatements)
		},
	)
}

func (b *Builder) addDropUnknownServiceNameProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropUnknownServiceNameProcessor),
		func(tp *telemetryv1beta1.TracePipeline) any {
			if opts.ServiceEnrichment != commonresources.AnnotationValueTelemetryServiceEnrichmentOtel {
				return nil // Kyma legacy enrichment selected, skip this processor
			}

			return common.TraceTransformProcessorConfig(common.DropUnknownServiceNameProcessorStatements())
		},
	)
}

func (b *Builder) addK8sAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDK8sAttributesProcessor),
		func(tp *telemetryv1beta1.TracePipeline) any {
			useOTelServiceEnrichment := opts.ServiceEnrichment == commonresources.AnnotationValueTelemetryServiceEnrichmentOtel
			return common.K8sAttributesProcessorConfig(opts.Enrichments, useOTelServiceEnrichment)
		},
	)
}

func (b *Builder) addIstioNoiseFilterProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDIstioNoiseFilterProcessor),
		func(tp *telemetryv1beta1.TracePipeline) any {
			return &common.IstioNoiseFilterProcessor{}
		},
	)
}

func (b *Builder) addInsertClusterAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDInsertClusterAttributesProcessor),
		func(tp *telemetryv1beta1.TracePipeline) any {
			transformStatements := common.InsertClusterAttributesProcessorStatements(opts.Cluster)
			return common.TraceTransformProcessorConfig(transformStatements)
		},
	)
}

func (b *Builder) addServiceEnrichmentProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDServiceEnrichmentProcessor),
		func(tp *telemetryv1beta1.TracePipeline) any {
			if opts.ServiceEnrichment == commonresources.AnnotationValueTelemetryServiceEnrichmentOtel {
				return nil // OTel service enrichment selected, skip this processor
			}

			return common.ResolveServiceNameConfig()
		},
	)
}

func (b *Builder) addDropKymaAttributesProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropKymaAttributesProcessor),
		func(tp *telemetryv1beta1.TracePipeline) any {
			transformStatements := common.DropKymaAttributesProcessorStatements()
			return common.TraceTransformProcessorConfig(transformStatements)
		},
	)
}

// addUserDefinedTransformProcessor handles user-defined transform processors with dynamic component IDs
func (b *Builder) addUserDefinedTransformProcessor() buildComponentFunc {
	return b.AddProcessor(
		formatUserDefinedTransformProcessorID,
		func(tp *telemetryv1beta1.TracePipeline) any {
			if len(tp.Spec.Transforms) == 0 {
				return nil // No transforms, no processor needed
			}

			transformStatements := common.TransformSpecsToProcessorStatements(tp.Spec.Transforms)

			return common.TraceTransformProcessorConfig(transformStatements)
		},
	)
}

func (b *Builder) addUserDefinedFilterProcessor() buildComponentFunc {
	return b.AddProcessor(
		formatUserDefinedFilterProcessorID,
		func(tp *telemetryv1beta1.TracePipeline) any {
			if tp.Spec.Filters == nil {
				return nil // No filters, no processor needed
			}

			return common.FilterSpecsToTraceFilterProcessorConfig(tp.Spec.Filters)
		})
}

//nolint:mnd // hardcoded values
func (b *Builder) addBatchProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDBatchProcessor),
		func(_ *telemetryv1beta1.TracePipeline) any {
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
		func(ctx context.Context, tp *telemetryv1beta1.TracePipeline) (any, common.EnvVars, error) {
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

func (b *Builder) addOAuth2Extension(ctx context.Context, pipeline *telemetryv1beta1.TracePipeline) error {
	oauth2ExtensionID := common.OAuth2ExtensionID(pipeline.Name)

	oauth2ExtensionConfig, oauth2ExtensionEnvVars, err := common.NewOAuth2ExtensionConfigBuilder(
		b.Reader,
		pipeline.Spec.Output.OTLP.Authentication.OAuth2,
		pipeline.Name,
		common.SignalTypeTrace,
	).OAuth2ExtensionConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to build OAuth2 extension for pipeline %s: %w", pipeline.Name, err)
	}

	b.AddExtension(oauth2ExtensionID, oauth2ExtensionConfig, oauth2ExtensionEnvVars)

	return nil
}

func shouldEnableOAuth2(tp *telemetryv1beta1.TracePipeline) bool {
	return tp.Spec.Output.OTLP.Authentication != nil && tp.Spec.Output.OTLP.Authentication.OAuth2 != nil
}

func formatTraceServicePipelineID(tp *telemetryv1beta1.TracePipeline) string {
	return fmt.Sprintf("traces/%s", tp.Name)
}

func formatUserDefinedTransformProcessorID(tp *telemetryv1beta1.TracePipeline) string {
	return fmt.Sprintf(common.ComponentIDUserDefinedTransformProcessor, tp.Name)
}

func formatUserDefinedFilterProcessorID(tp *telemetryv1beta1.TracePipeline) string {
	return fmt.Sprintf(common.ComponentIDUserDefinedFilterProcessor, tp.Name)
}

func formatOTLPExporterID(tp *telemetryv1beta1.TracePipeline) string {
	return common.ExporterID(tp.Spec.Output.OTLP.Protocol, tp.Name)
}
