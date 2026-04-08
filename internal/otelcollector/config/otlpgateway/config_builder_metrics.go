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

// buildMetricPipelines builds metric pipeline configuration and adds it to the shared config.
// Unlike trace/log which use flat per-pipeline pipelines, metrics use a 3-stage architecture:
// input pipelines → enrichment pipeline → per-pipeline output pipelines (connected via forward connectors).
func (b *Builder) buildMetricPipelines(ctx context.Context, builder *common.ComponentBuilder[*telemetryv1beta1.MetricPipeline], opts BuildOptions) error {
	pipelines := opts.MetricPipelines
	if len(pipelines) == 0 {
		return nil
	}

	// Add leader election extension for KymaStats receiver
	builder.AddExtension(common.ComponentIDK8sLeaderElectorExtension,
		common.K8sLeaderElectorExtensionConfig{
			AuthType:       "serviceAccount",
			LeaseName:      common.K8sLeaderElectorKymaStats,
			LeaseNamespace: opts.GatewayNamespace,
		},
		nil,
	)

	queueSize := common.BatchingMaxQueueSize / len(pipelines)

	// Input pipeline: OTLP receiver
	if err := builder.AddServicePipeline(ctx, nil, "metrics/input-otlp",
		b.addMetricOTLPReceiver(builder),
		b.addMetricSetKymaInputNameProcessor(builder, common.InputSourceOTLP),
		b.addMetricExporterForInputForwarder(builder),
	); err != nil {
		return fmt.Errorf("failed to add metric input-otlp service pipeline: %w", err)
	}

	// Input pipeline: KymaStats receiver
	if err := builder.AddServicePipeline(ctx, nil, "metrics/input-kyma-stats",
		b.addMetricKymaStatsReceiver(builder),
		b.addMetricSetKymaInputNameProcessor(builder, common.InputSourceKyma),
		b.addMetricExporterForInputForwarder(builder),
	); err != nil {
		return fmt.Errorf("failed to add metric input-kyma-stats service pipeline: %w", err)
	}

	// Enrichment pipeline
	if err := builder.AddServicePipeline(ctx, nil, "metrics/enrichment",
		b.addMetricReceiverForInputForwarder(builder),
		b.addMetricMemoryLimiterProcessor(builder),
		b.addMetricSetInstrumentationScopeToKymaProcessor(builder, opts),
		b.addMetricDropUnknownServiceNameProcessor(builder, opts),
		b.addMetricK8sAttributesProcessor(builder, opts),
		b.addMetricServiceEnrichmentProcessor(builder, opts),
		b.addMetricInsertClusterAttributesProcessor(builder, opts),
		b.addMetricExporterForEnrichmentForwarder(builder),
	); err != nil {
		return fmt.Errorf("failed to add metric enrichment service pipeline: %w", err)
	}

	// Per-pipeline output pipelines
	for _, pipeline := range pipelines {
		outputPipelineID := formatMetricOutputServicePipelineID(&pipeline)

		if shouldEnableMetricOAuth2(&pipeline) {
			if err := b.addMetricOAuth2Extension(ctx, builder, &pipeline); err != nil {
				return err
			}
		}

		if err := builder.AddServicePipeline(ctx, &pipeline, outputPipelineID,
			b.addMetricReceiverForEnrichmentForwarder(builder),
			b.addMetricDropOTLPIfInputDisabledProcessor(builder),
			b.addMetricOTLPNamespaceFilterProcessor(builder),
			b.addMetricDropKymaAttributesProcessor(builder),
			b.addMetricUserDefinedTransformProcessor(builder),
			b.addMetricUserDefinedFilterProcessor(builder),
			b.addMetricBatchProcessor(builder),
			b.addMetricOTLPExporter(builder, queueSize),
		); err != nil {
			return fmt.Errorf("failed to add metric output service pipeline: %w", err)
		}
	}

	return nil
}

// ======================================================
// Input pipeline components
// ======================================================

func (b *Builder) addMetricOTLPReceiver(builder *common.ComponentBuilder[*telemetryv1beta1.MetricPipeline]) buildMetricComponentFunc {
	return builder.AddReceiver(
		builder.StaticComponentID(common.ComponentIDOTLPReceiver),
		func(mp *telemetryv1beta1.MetricPipeline) any {
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

func (b *Builder) addMetricKymaStatsReceiver(builder *common.ComponentBuilder[*telemetryv1beta1.MetricPipeline]) buildMetricComponentFunc {
	return builder.AddReceiver(
		builder.StaticComponentID(common.ComponentIDKymaStatsReceiver),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			return &KymaStatsReceiverConfig{
				AuthType:           "serviceAccount",
				K8sLeaderElector:   "k8s_leader_elector",
				CollectionInterval: "30s",
				Resources: []ModuleGVR{
					{Group: "operator.kyma-project.io", Version: "v1beta1", Resource: "telemetries"},
					{Group: "telemetry.kyma-project.io", Version: "v1beta1", Resource: "logpipelines"},
					{Group: "telemetry.kyma-project.io", Version: "v1beta1", Resource: "tracepipelines"},
					{Group: "telemetry.kyma-project.io", Version: "v1beta1", Resource: "metricpipelines"},
				},
			}
		},
	)
}

func (b *Builder) addMetricSetKymaInputNameProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.MetricPipeline], inputSource common.InputSourceType) buildMetricComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.InputName[inputSource]),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			transformStatements := common.KymaInputNameProcessorStatements(inputSource)
			return common.MetricTransformProcessor(transformStatements)
		},
	)
}

func (b *Builder) addMetricExporterForInputForwarder(builder *common.ComponentBuilder[*telemetryv1beta1.MetricPipeline]) buildMetricComponentFunc {
	return builder.AddExporter(
		builder.StaticComponentID(common.ComponentIDInputConnector),
		func(ctx context.Context, mp *telemetryv1beta1.MetricPipeline) (any, common.EnvVars, error) {
			return &common.ForwardConnectorConfig{}, nil, nil
		},
	)
}

func (b *Builder) addMetricReceiverForInputForwarder(builder *common.ComponentBuilder[*telemetryv1beta1.MetricPipeline]) buildMetricComponentFunc {
	return builder.AddReceiver(
		builder.StaticComponentID(common.ComponentIDInputConnector),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			return &common.ForwardConnectorConfig{}
		},
	)
}

// ======================================================
// Enrichment pipeline components
// ======================================================

//nolint:mnd // hardcoded values
func (b *Builder) addMetricMemoryLimiterProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.MetricPipeline]) buildMetricComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDMemoryLimiterProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			return &common.MemoryLimiterConfig{
				CheckInterval:        "1s",
				LimitPercentage:      75,
				SpikeLimitPercentage: 15,
			}
		},
	)
}

func (b *Builder) addMetricSetInstrumentationScopeToKymaProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.MetricPipeline], opts BuildOptions) buildMetricComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDSetInstrumentationScopeKymaProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			return common.InstrumentationScopeProcessor(opts.ModuleVersion, common.InputSourceKyma)
		},
	)
}

func (b *Builder) addMetricDropUnknownServiceNameProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.MetricPipeline], opts BuildOptions) buildMetricComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDDropUnknownServiceNameProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if opts.ServiceEnrichment != commonresources.AnnotationValueTelemetryServiceEnrichmentOtel {
				return nil
			}

			return common.MetricTransformProcessor(common.DropUnknownServiceNameProcessorStatements())
		},
	)
}

func (b *Builder) addMetricK8sAttributesProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.MetricPipeline], opts BuildOptions) buildMetricComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDK8sAttributesProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			useOTelServiceEnrichment := opts.ServiceEnrichment == commonresources.AnnotationValueTelemetryServiceEnrichmentOtel
			return common.K8sAttributesProcessor(opts.Enrichments, useOTelServiceEnrichment)
		},
	)
}

func (b *Builder) addMetricServiceEnrichmentProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.MetricPipeline], opts BuildOptions) buildMetricComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDServiceEnrichmentProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if opts.ServiceEnrichment == commonresources.AnnotationValueTelemetryServiceEnrichmentOtel {
				return nil
			}

			return common.ResolveServiceName()
		},
	)
}

func (b *Builder) addMetricInsertClusterAttributesProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.MetricPipeline], opts BuildOptions) buildMetricComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDInsertClusterAttributesProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			transformStatements := common.InsertClusterAttributesProcessorStatements(opts.Cluster)
			return common.MetricTransformProcessor(transformStatements)
		},
	)
}

func (b *Builder) addMetricExporterForEnrichmentForwarder(builder *common.ComponentBuilder[*telemetryv1beta1.MetricPipeline]) buildMetricComponentFunc {
	return builder.AddExporter(
		builder.StaticComponentID(common.ComponentIDEnrichmentConnector),
		func(ctx context.Context, mp *telemetryv1beta1.MetricPipeline) (any, common.EnvVars, error) {
			return &common.ForwardConnectorConfig{}, nil, nil
		},
	)
}

// ======================================================
// Output pipeline components
// ======================================================

func (b *Builder) addMetricReceiverForEnrichmentForwarder(builder *common.ComponentBuilder[*telemetryv1beta1.MetricPipeline]) buildMetricComponentFunc {
	return builder.AddReceiver(
		builder.StaticComponentID(common.ComponentIDEnrichmentConnector),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			return &common.ForwardConnectorConfig{}
		},
	)
}

func (b *Builder) addMetricDropOTLPIfInputDisabledProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.MetricPipeline]) buildMetricComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDDropIfInputSourceOTLPProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if sharedtypesutils.IsOTLPInputEnabled(mp.Spec.Input.OTLP) {
				return nil
			}

			return common.MetricFilterProcessor([]telemetryv1beta1.FilterSpec{
				{
					Conditions: []string{common.KymaInputNameEquals(common.InputSourceOTLP)},
				},
			})
		},
	)
}

func (b *Builder) addMetricOTLPNamespaceFilterProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.MetricPipeline]) buildMetricComponentFunc {
	return builder.AddProcessor(
		formatMetricOTLPNamespaceFilterID,
		func(mp *telemetryv1beta1.MetricPipeline) any {
			input := mp.Spec.Input
			if !sharedtypesutils.IsOTLPInputEnabled(input.OTLP) || input.OTLP == nil || !metricShouldFilterByNamespace(input.OTLP.Namespaces) {
				return nil
			}

			return metricFilterByNamespaceProcessorConfig(input.OTLP.Namespaces)
		},
	)
}

func (b *Builder) addMetricDropKymaAttributesProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.MetricPipeline]) buildMetricComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDDropKymaAttributesProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			transformStatements := common.DropKymaAttributesProcessorStatements()
			return common.MetricTransformProcessor(transformStatements)
		},
	)
}

func (b *Builder) addMetricUserDefinedTransformProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.MetricPipeline]) buildMetricComponentFunc {
	return builder.AddProcessor(
		formatMetricUserDefinedTransformProcessorID,
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if len(mp.Spec.Transforms) == 0 {
				return nil
			}

			transformStatements := common.TransformSpecsToProcessorStatements(mp.Spec.Transforms)

			return common.MetricTransformProcessor(transformStatements)
		},
	)
}

func (b *Builder) addMetricUserDefinedFilterProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.MetricPipeline]) buildMetricComponentFunc {
	return builder.AddProcessor(
		formatMetricUserDefinedFilterProcessorID,
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if mp.Spec.Filters == nil {
				return nil
			}

			return common.MetricFilterProcessor(mp.Spec.Filters)
		},
	)
}

//nolint:mnd // hardcoded values
func (b *Builder) addMetricBatchProcessor(builder *common.ComponentBuilder[*telemetryv1beta1.MetricPipeline]) buildMetricComponentFunc {
	return builder.AddProcessor(
		builder.StaticComponentID(common.ComponentIDBatchProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			return &common.BatchProcessorConfig{
				SendBatchSize:    1024,
				Timeout:          "10s",
				SendBatchMaxSize: 1024,
			}
		},
	)
}

func (b *Builder) addMetricOTLPExporter(builder *common.ComponentBuilder[*telemetryv1beta1.MetricPipeline], queueSize int) buildMetricComponentFunc {
	return builder.AddExporter(
		formatMetricOTLPExporterID,
		func(ctx context.Context, mp *telemetryv1beta1.MetricPipeline) (any, common.EnvVars, error) {
			otlpExporterBuilder := common.NewOTLPExporterConfigBuilder(
				b.Reader,
				mp.Spec.Output.OTLP,
				common.MetricPipelineRef(mp),
				queueSize,
			)

			return otlpExporterBuilder.OTLPExporter(ctx)
		},
	)
}

// ======================================================
// Authentication extensions
// ======================================================

//nolint:dupl // Acceptable duplication - metric, trace and log OAuth2 extensions follow same pattern
func (b *Builder) addMetricOAuth2Extension(ctx context.Context, builder *common.ComponentBuilder[*telemetryv1beta1.MetricPipeline], pipeline *telemetryv1beta1.MetricPipeline) error {
	pipelineRef := common.MetricPipelineRef(pipeline)
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

// ======================================================
// Helper functions
// ======================================================

func metricShouldFilterByNamespace(namespaceSelector *telemetryv1beta1.NamespaceSelector) bool {
	return namespaceSelector != nil && (len(namespaceSelector.Include) > 0 || len(namespaceSelector.Exclude) > 0)
}

func metricFilterByNamespaceProcessorConfig(namespaceSelector *telemetryv1beta1.NamespaceSelector) *common.FilterProcessorConfig {
	var filterExpressions []string

	notFromKymaStatsReceiver := common.Not(common.KymaInputNameEquals(common.InputSourceKyma))

	if len(namespaceSelector.Exclude) > 0 {
		namespacesConditions := metricNamespacesConditionsBuilder(namespaceSelector.Exclude)
		excludeNamespacesExpr := common.JoinWithAnd(notFromKymaStatsReceiver, common.JoinWithOr(namespacesConditions...))
		filterExpressions = append(filterExpressions, excludeNamespacesExpr)
	}

	if len(namespaceSelector.Include) > 0 {
		namespacesConditions := metricNamespacesConditionsBuilder(namespaceSelector.Include)
		includeNamespacesExpr := common.JoinWithAnd(
			notFromKymaStatsReceiver,
			common.ResourceAttributeIsNotNil(common.K8sNamespaceName),
			common.Not(common.JoinWithOr(namespacesConditions...)),
		)
		filterExpressions = append(filterExpressions, includeNamespacesExpr)
	}

	return common.MetricFilterProcessor([]telemetryv1beta1.FilterSpec{
		{
			Conditions: filterExpressions,
		},
	})
}

func metricNamespacesConditionsBuilder(namespaces []string) []string {
	var conditions []string
	for _, ns := range namespaces {
		conditions = append(conditions, common.NamespaceEquals(ns))
	}

	return conditions
}

func formatMetricOutputServicePipelineID(mp *telemetryv1beta1.MetricPipeline) string {
	return fmt.Sprintf("metrics/%s-output", mp.Name)
}

func formatMetricOTLPNamespaceFilterID(mp *telemetryv1beta1.MetricPipeline) string {
	return common.ComponentIDNamespacePerInputFilterProcessor(mp.Name, common.InputSourceOTLP)
}

func formatMetricOTLPExporterID(pipeline *telemetryv1beta1.MetricPipeline) string {
	return common.ComponentIDOTLPExporter(pipeline.Spec.Output.OTLP.Protocol, common.MetricPipelineRef(pipeline))
}

func formatMetricUserDefinedTransformProcessorID(mp *telemetryv1beta1.MetricPipeline) string {
	ref := common.MetricPipelineRef(mp)
	return common.ComponentIDUserDefinedTransformProcessor(ref.TypePrefix(), ref.Name())
}

func formatMetricUserDefinedFilterProcessorID(mp *telemetryv1beta1.MetricPipeline) string {
	ref := common.MetricPipelineRef(mp)
	return common.ComponentIDUserDefinedFilterProcessor(ref.TypePrefix(), ref.Name())
}

func shouldEnableMetricOAuth2(mp *telemetryv1beta1.MetricPipeline) bool {
	return mp.Spec.Output.OTLP.Authentication != nil && mp.Spec.Output.OTLP.Authentication.OAuth2 != nil
}
