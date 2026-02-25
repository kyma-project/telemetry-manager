package otlpgateway

import (
	"context"
	"fmt"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	sharedtypesutils "github.com/kyma-project/telemetry-manager/internal/utils/sharedtypes"
)

// buildLogPipelines builds log pipeline configuration and adds it to the shared config.
func (b *Builder) buildLogPipelines(ctx context.Context, builder *common.ComponentBuilder[*telemetryv1beta1.LogPipeline], opts BuildOptions) error {
	pipelines := opts.LogPipelines
	if len(pipelines) == 0 {
		return nil
	}

	queueSize := common.BatchingMaxQueueSize / len(pipelines)

	for _, pipeline := range pipelines {
		pipelineID := formatLogServicePipelineID(&pipeline)

		if shouldEnableLogOAuth2(&pipeline) {
			if err := b.addLogOAuth2Extension(ctx, builder, &pipeline); err != nil {
				return err
			}
		}

		if err := builder.AddServicePipeline(ctx, &pipeline, pipelineID,
			b.addLogOTLPReceiver(builder),
			b.addLogMemoryLimiterProcessor(builder),
			b.addSetObsTimeIfZeroProcessor(builder),
			b.addLogDropUnknownServiceNameProcessor(builder, opts),
			b.addLogK8sAttributesProcessor(builder, opts),
			b.addLogIstioNoiseFilterProcessor(builder),
			b.addDropIfInputSourceOTLPProcessor(builder),
			b.addNamespaceFilterProcessor(builder),
			b.addLogInsertClusterAttributesProcessor(builder, opts),
			b.addLogServiceEnrichmentProcessor(builder, opts),
			b.addLogDropKymaAttributesProcessor(builder),
			b.addLogIstioAccessLogsEnrichmentProcessor(builder, opts),
			b.addLogUserDefinedTransformProcessor(builder),
			b.addLogUserDefinedFilterProcessor(builder),
			b.addLogBatchProcessor(builder),
			b.addLogOTLPExporter(builder, queueSize),
		); err != nil {
			return fmt.Errorf("failed to add log service pipeline: %w", err)
		}
	}

	return nil
}

func (b *Builder) addLogOTLPReceiver(builder *common.ComponentBuilder[*telemetryv1beta1.LogPipeline]) buildLogComponentFunc {
	return builder.AddReceiver(
		builder.StaticComponentID(common.ComponentIDOTLPReceiver),
		func(lp *telemetryv1beta1.LogPipeline) any {
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
func (b *Builder) addLogMemoryLimiterProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.LogPipeline]) buildLogComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDMemoryLimiterProcessor),
		func(lp *telemetryv1beta1.LogPipeline) any {
			return &common.MemoryLimiter{
				CheckInterval:        "1s",
				LimitPercentage:      75,
				SpikeLimitPercentage: 15,
			}
		},
	)
}

func (b *Builder) addSetObsTimeIfZeroProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.LogPipeline]) buildLogComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDSetObservedTimeIfZeroProcessor),
		func(lp *telemetryv1beta1.LogPipeline) any {
			return common.LogTransformProcessorConfig([]common.TransformProcessorStatements{{
				Conditions: []string{"log.observed_time_unix_nano == 0"},
				Statements: []string{"set(log.observed_time, Now())"},
			}})
		},
	)
}

func (b *Builder) addLogDropUnknownServiceNameProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.LogPipeline], opts BuildOptions) buildLogComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDDropUnknownServiceNameProcessor),
		func(lp *telemetryv1beta1.LogPipeline) any {
			if opts.ServiceEnrichment != commonresources.AnnotationValueTelemetryServiceEnrichmentOtel {
				return nil // Kyma legacy enrichment selected, skip this processor
			}

			return common.LogTransformProcessorConfig(common.DropUnknownServiceNameProcessorStatements())
		},
	)
}

func (b *Builder) addLogK8sAttributesProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.LogPipeline], opts BuildOptions) buildLogComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDK8sAttributesProcessor),
		func(lp *telemetryv1beta1.LogPipeline) any {
			useOTelServiceEnrichment := opts.ServiceEnrichment == commonresources.AnnotationValueTelemetryServiceEnrichmentOtel
			return common.K8sAttributesProcessorConfig(opts.Enrichments, useOTelServiceEnrichment)
		},
	)
}

func (b *Builder) addLogIstioNoiseFilterProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.LogPipeline]) buildLogComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDIstioNoiseFilterProcessor),
		func(lp *telemetryv1beta1.LogPipeline) any {
			return &common.IstioNoiseFilterProcessor{}
		},
	)
}

func (b *Builder) addDropIfInputSourceOTLPProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.LogPipeline]) buildLogComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDDropIfInputSourceOTLPProcessor),
		func(lp *telemetryv1beta1.LogPipeline) any {
			if sharedtypesutils.IsOTLPInputEnabled(lp.Spec.Input.OTLP) {
				return nil // Skip this processor if OTLP input is enabled
			}

			return dropIfInputSourceOTLPProcessorConfig()
		},
	)
}

func (b *Builder) addNamespaceFilterProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.LogPipeline]) buildLogComponentFunc {
	return builder.AddProcessor(
		formatNamespaceFilterID,
		func(lp *telemetryv1beta1.LogPipeline) any {
			otlpInput := lp.Spec.Input.OTLP
			if otlpInput == nil || !sharedtypesutils.IsOTLPInputEnabled(otlpInput) || !shouldFilterByNamespace(otlpInput.Namespaces) {
				return nil // No namespace filter needed
			}

			return namespaceFilterProcessorConfig(otlpInput.Namespaces)
		},
	)
}

func (b *Builder) addLogInsertClusterAttributesProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.LogPipeline], opts BuildOptions) buildLogComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDInsertClusterAttributesProcessor),
		func(lp *telemetryv1beta1.LogPipeline) any {
			transformStatements := common.InsertClusterAttributesProcessorStatements(opts.Cluster)
			return common.LogTransformProcessorConfig(transformStatements)
		},
	)
}

func (b *Builder) addLogServiceEnrichmentProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.LogPipeline], opts BuildOptions) buildLogComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDServiceEnrichmentProcessor),
		func(lp *telemetryv1beta1.LogPipeline) any {
			if opts.ServiceEnrichment == commonresources.AnnotationValueTelemetryServiceEnrichmentOtel {
				return nil // OTel service enrichment selected, skip this processor
			}

			return common.ResolveServiceNameConfig()
		},
	)
}

func (b *Builder) addLogDropKymaAttributesProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.LogPipeline]) buildLogComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDDropKymaAttributesProcessor),
		func(lp *telemetryv1beta1.LogPipeline) any {
			transformStatements := common.DropKymaAttributesProcessorStatements()
			return common.LogTransformProcessorConfig(transformStatements)
		},
	)
}

func (b *Builder) addLogIstioAccessLogsEnrichmentProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.LogPipeline], opts BuildOptions) buildLogComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDIstioEnrichmentProcessor),
		func(lp *telemetryv1beta1.LogPipeline) any {
			return &IstioEnrichmentProcessor{
				ScopeVersion: opts.ModuleVersion,
			}
		},
	)
}

func (b *Builder) addLogUserDefinedTransformProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.LogPipeline]) buildLogComponentFunc {
	return builder.AddProcessor(
		formatLogUserDefinedTransformProcessorID,
		func(lp *telemetryv1beta1.LogPipeline) any {
			if len(lp.Spec.Transforms) == 0 {
				return nil // No transforms, no processor needed
			}

			transformStatements := common.TransformSpecsToProcessorStatements(lp.Spec.Transforms)

			return common.LogTransformProcessorConfig(transformStatements)
		},
	)
}

func (b *Builder) addLogUserDefinedFilterProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.LogPipeline]) buildLogComponentFunc {
	return builder.AddProcessor(
		formatLogUserDefinedFilterProcessorID,
		func(lp *telemetryv1beta1.LogPipeline) any {
			if lp.Spec.Filters == nil {
				return nil // No filters, no processor needed
			}

			return common.FilterSpecsToLogFilterProcessorConfig(lp.Spec.Filters)
		},
	)
}

//nolint:mnd // hardcoded values
func (b *Builder) addLogBatchProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.LogPipeline]) buildLogComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDBatchProcessor),
		func(lp *telemetryv1beta1.LogPipeline) any {
			return &common.BatchProcessor{
				SendBatchSize:    512,
				Timeout:          "10s",
				SendBatchMaxSize: 512,
			}
		},
	)
}

func (b *Builder) addLogOTLPExporter(builder *common.ComponentBuilder[*telemetryv1beta1.LogPipeline], queueSize int) buildLogComponentFunc {
	return builder.AddExporter(
		formatLogOTLPExporterID,
		func(ctx context.Context, lp *telemetryv1beta1.LogPipeline) (any, common.EnvVars, error) {
			otlpExporterBuilder := common.NewOTLPExporterConfigBuilder(
				b.Reader,
				lp.Spec.Output.OTLP,
				lp.Name,
				queueSize,
				common.SignalTypeLog,
			)

			return otlpExporterBuilder.OTLPExporterConfig(ctx)
		},
	)
}

//nolint:dupl // Acceptable duplication - trace and log OAuth2 extensions follow same pattern
func (b *Builder) addLogOAuth2Extension(ctx context.Context, builder *common.ComponentBuilder[*telemetryv1beta1.LogPipeline], pipeline *telemetryv1beta1.LogPipeline) error {
	oauth2ExtensionID := common.OAuth2ExtensionID(pipeline.Name)

	oauth2ExtensionConfig, oauth2ExtensionEnvVars, err := common.NewOAuth2ExtensionConfigBuilder(
		b.Reader,
		pipeline.Spec.Output.OTLP.Authentication.OAuth2,
		pipeline.Name,
		common.SignalTypeLog,
	).OAuth2ExtensionConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to build OAuth2 extension for pipeline %s: %w", pipeline.Name, err)
	}

	builder.AddExtension(oauth2ExtensionID, oauth2ExtensionConfig, oauth2ExtensionEnvVars)

	return nil
}

// Log pipeline helper functions

func shouldEnableLogOAuth2(lp *telemetryv1beta1.LogPipeline) bool {
	return lp.Spec.Output.OTLP.Authentication != nil && lp.Spec.Output.OTLP.Authentication.OAuth2 != nil
}

func formatLogServicePipelineID(lp *telemetryv1beta1.LogPipeline) string {
	return fmt.Sprintf("logs/%s", lp.Name)
}

func formatNamespaceFilterID(lp *telemetryv1beta1.LogPipeline) string {
	return fmt.Sprintf(common.ComponentIDNamespaceFilterProcessor, lp.Name)
}

func formatLogUserDefinedTransformProcessorID(lp *telemetryv1beta1.LogPipeline) string {
	return fmt.Sprintf(common.ComponentIDUserDefinedTransformProcessor, lp.Name)
}

func formatLogUserDefinedFilterProcessorID(lp *telemetryv1beta1.LogPipeline) string {
	return fmt.Sprintf(common.ComponentIDUserDefinedFilterProcessor, lp.Name)
}

func formatLogOTLPExporterID(lp *telemetryv1beta1.LogPipeline) string {
	return common.ExporterID(lp.Spec.Output.OTLP.Protocol, lp.Name)
}

func namespaceFilterProcessorConfig(namespaceSelector *telemetryv1beta1.NamespaceSelector) *common.FilterProcessor {
	var filterExpressions []string

	if len(namespaceSelector.Exclude) > 0 {
		namespacesConditions := namespacesConditions(namespaceSelector.Exclude)

		// Drop logs if the excluded namespaces are matched
		excludeNamespacesExpr := common.JoinWithOr(namespacesConditions...)
		filterExpressions = append(filterExpressions, excludeNamespacesExpr)
	}

	if len(namespaceSelector.Include) > 0 {
		namespacesConditions := namespacesConditions(namespaceSelector.Include)
		includeNamespacesExpr := common.JoinWithAnd(
			// Ensure the k8s.namespace.name resource attribute is not nil,
			// so we don't drop logs without a namespace label
			common.ResourceAttributeIsNotNil(common.K8sNamespaceName),

			// Logs are dropped if the filter expression evaluates to true,
			// so we negate the match against included namespaces to keep only those
			common.Not(common.JoinWithOr(namespacesConditions...)),
		)
		filterExpressions = append(filterExpressions, includeNamespacesExpr)
	}

	return common.FilterSpecsToLogFilterProcessorConfig([]telemetryv1beta1.FilterSpec{{Conditions: filterExpressions}})
}

func namespacesConditions(namespaces []string) []string {
	var conditions []string
	for _, ns := range namespaces {
		conditions = append(conditions, common.NamespaceEquals(ns))
	}

	return conditions
}

func dropIfInputSourceOTLPProcessorConfig() *common.FilterProcessor {
	return common.FilterSpecsToLogFilterProcessorConfig([]telemetryv1beta1.FilterSpec{
		{Conditions: []string{
			common.JoinWithOr(common.IsNotNil("log.observed_time"), common.IsNotNil("log.time")),
		}},
	})
}

func shouldFilterByNamespace(namespaceSelector *telemetryv1beta1.NamespaceSelector) bool {
	return namespaceSelector != nil && (len(namespaceSelector.Include) > 0 || len(namespaceSelector.Exclude) > 0)
}
