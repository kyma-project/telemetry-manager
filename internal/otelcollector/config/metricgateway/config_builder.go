package metricgateway

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	sharedtypesutils "github.com/kyma-project/telemetry-manager/internal/utils/sharedtypes"
)

type buildComponentFunc = common.BuildComponentFunc[*telemetryv1beta1.MetricPipeline]

type Builder struct {
	common.ComponentBuilder[*telemetryv1beta1.MetricPipeline]

	Reader client.Reader
}

type BuildOptions struct {
	Cluster                     common.ClusterOptions
	GatewayNamespace            string
	InstrumentationScopeVersion string
	Enrichments                 *operatorv1beta1.EnrichmentSpec
	// ServiceEnrichment specifies the service enrichment strategy to be used (temporary)
	ServiceEnrichment string
}

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1beta1.MetricPipeline, opts BuildOptions) (*common.Config, common.EnvVars, error) {
	b.Config = common.NewConfig()
	b.AddExtension(common.ComponentIDK8sLeaderElectorExtension,
		common.K8sLeaderElectorExtension{
			AuthType:       "serviceAccount",
			LeaseName:      common.K8sLeaderElectorKymaStats,
			LeaseNamespace: opts.GatewayNamespace,
		},
		nil,
	)
	b.EnvVars = make(common.EnvVars)

	queueSize := common.BatchingMaxQueueSize / len(pipelines)

	// split the service pipeline into two input pipelines and one actual service pipeline
	if err := b.AddServicePipeline(ctx, nil, "metrics/input-otlp",
		b.addOTLPReceiver(),
		b.addSetKymaInputNameProcessor(common.InputSourceOTLP),
		b.addExporterForInputForwarder(),
	); err != nil {
		return nil, nil, fmt.Errorf("failed to add input service pipeline: %w", err)
	}

	if err := b.AddServicePipeline(ctx, nil, "metrics/input-kyma-stats",
		b.addKymaStatsReceiver(),
		b.addSetKymaInputNameProcessor(common.InputSourceKyma),
		b.addExporterForInputForwarder(),
	); err != nil {
		return nil, nil, fmt.Errorf("failed to add input service pipeline: %w", err)
	}

	if err := b.AddServicePipeline(ctx, nil, "metrics/enrichment",
		b.addReceiverForInputForwarder(),
		b.addMemoryLimiterProcessor(),
		b.addSetInstrumentationScopeToKymaProcessor(opts),
		b.addDropUnknownServiceNameProcessor(opts),
		b.addK8sAttributesProcessor(opts),
		b.addServiceEnrichmentProcessor(opts),
		b.addInsertClusterAttributesProcessor(opts),
		b.addExporterForEnrichmentForwarder(),
	); err != nil {
		return nil, nil, fmt.Errorf("failed to add input service pipeline: %w", err)
	}

	for _, pipeline := range pipelines {
		outputPipelineID := formatOutputServicePipelineID(&pipeline)

		if shouldEnableOAuth2(&pipeline) {
			if err := b.addOAuth2Extension(ctx, &pipeline); err != nil {
				return nil, nil, err
			}
		}

		if err := b.AddServicePipeline(ctx, &pipeline, outputPipelineID,
			b.addReceiverForEnrichmentForwarder(),
			// Input source filters if otlp is disabled
			b.addDropOTLPIfInputDisabledProcessor(),
			// Namespace filters
			b.addOTLPNamespaceFilterProcessor(),
			// Kyma attributes are dropped before user-defined transform and filter processors
			// to prevent user access to internal attributes.
			b.addDropKymaAttributesProcessor(),
			// User defined Transform and Filter
			b.addUserDefinedTransformProcessor(),
			b.addUserDefinedFilterProcessor(),
			// Batch processor (always last)
			b.addBatchProcessor(),
			// OTLP exporter
			b.addOTLPExporter(queueSize),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add output service pipeline: %w", err)
		}
	}

	return b.Config, b.EnvVars, nil
}

// ======================================================
// Input pipeline components
// ======================================================
func (b *Builder) addOTLPReceiver() buildComponentFunc {
	return b.AddReceiver(
		b.StaticComponentID(common.ComponentIDOTLPReceiver),
		func(mp *telemetryv1beta1.MetricPipeline) any {
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

func (b *Builder) addKymaStatsReceiver() buildComponentFunc {
	return b.AddReceiver(
		b.StaticComponentID(common.ComponentIDKymaStatsReceiver),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			return &KymaStatsReceiver{
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

func (b *Builder) addSetKymaInputNameProcessor(inputSource common.InputSourceType) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.InputName[inputSource]),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			transformStatements := common.KymaInputNameProcessorStatements(inputSource)
			return common.MetricTransformProcessorConfig(transformStatements)
		},
	)
}

//nolint:mnd // hardcoded values
func (b *Builder) addMemoryLimiterProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDMemoryLimiterProcessor),
		func(lp *telemetryv1beta1.MetricPipeline) any {
			return &common.MemoryLimiter{
				CheckInterval:        "1s",
				LimitPercentage:      75,
				SpikeLimitPercentage: 15,
			}
		},
	)
}

func (b *Builder) addSetInstrumentationScopeToKymaProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDSetInstrumentationScopeKymaProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			return common.InstrumentationScopeProcessorConfig(opts.InstrumentationScopeVersion, common.InputSourceKyma)
		},
	)
}

func (b *Builder) addDropUnknownServiceNameProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropUnknownServiceNameProcessor),
		func(tp *telemetryv1beta1.MetricPipeline) any {
			if opts.ServiceEnrichment != commonresources.AnnotationValueTelemetryServiceEnrichmentOtel {
				return nil // Kyma legacy enrichment selected, skip this processor
			}

			return common.MetricTransformProcessorConfig(common.DropUnknownServiceNameProcessorStatements())
		},
	)
}

func (b *Builder) addK8sAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDK8sAttributesProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			useOTelServiceEnrichment := opts.ServiceEnrichment == commonresources.AnnotationValueTelemetryServiceEnrichmentOtel
			return common.K8sAttributesProcessorConfig(opts.Enrichments, useOTelServiceEnrichment)
		},
	)
}

func (b *Builder) addServiceEnrichmentProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDServiceEnrichmentProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if opts.ServiceEnrichment == commonresources.AnnotationValueTelemetryServiceEnrichmentOtel {
				return nil // OTel service enrichment selected, skip this processor
			}

			return common.ResolveServiceNameConfig()
		},
	)
}

// Resource processors

func (b *Builder) addInsertClusterAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDInsertClusterAttributesProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			transformStatements := common.InsertClusterAttributesProcessorStatements(opts.Cluster)
			return common.MetricTransformProcessorConfig(transformStatements)
		},
	)
}

func (b *Builder) addExporterForEnrichmentForwarder() buildComponentFunc {
	return b.AddExporter(
		b.StaticComponentID(common.ComponentIDEnrichmentConnector),
		func(ctx context.Context, mp *telemetryv1beta1.MetricPipeline) (any, common.EnvVars, error) {
			return &common.ForwardConnector{}, nil, nil
		},
	)
}

func (b *Builder) addExporterForInputForwarder() buildComponentFunc {
	return b.AddExporter(
		b.StaticComponentID(common.ComponentIDInputConnector),
		func(ctx context.Context, mp *telemetryv1beta1.MetricPipeline) (any, common.EnvVars, error) {
			return &common.ForwardConnector{}, nil, nil
		},
	)
}

// ======================================================
// Output pipeline components
// ======================================================

func (b *Builder) addReceiverForEnrichmentForwarder() buildComponentFunc {
	return b.AddReceiver(
		b.StaticComponentID(common.ComponentIDEnrichmentConnector),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			return &common.ForwardConnector{}
		},
	)
}

func (b *Builder) addReceiverForInputForwarder() buildComponentFunc {
	return b.AddReceiver(
		b.StaticComponentID(common.ComponentIDInputConnector),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			return &common.ForwardConnector{}
		},
	)
}

// Input source filter processors

// if OTLP input is disabled filter drops all metrics where instrumentation scope is not 'kyma'
func (b *Builder) addDropOTLPIfInputDisabledProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropIfInputSourceOTLPProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if sharedtypesutils.IsOTLPInputEnabled(mp.Spec.Input.OTLP) {
				return nil
			}

			return common.MetricFilterProcessorConfig(common.FilterProcessorMetrics{
				Metric: []string{common.KymaInputNameEquals(common.InputSourceOTLP)},
			})
		},
	)
}

// Namespace filter processors

func (b *Builder) addOTLPNamespaceFilterProcessor() buildComponentFunc {
	return b.AddProcessor(
		func(mp *telemetryv1beta1.MetricPipeline) string {
			return formatOTLPNamespaceFilterID(mp.Name)
		},
		func(mp *telemetryv1beta1.MetricPipeline) any {
			input := mp.Spec.Input
			if !sharedtypesutils.IsOTLPInputEnabled(input.OTLP) || input.OTLP == nil || !shouldFilterByNamespace(input.OTLP.Namespaces) {
				return nil
			}

			return filterByNamespaceProcessorConfig(input.OTLP.Namespaces)
		},
	)
}

func (b *Builder) addDropKymaAttributesProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropKymaAttributesProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			transformStatements := common.DropKymaAttributesProcessorStatements()
			return common.MetricTransformProcessorConfig(transformStatements)
		},
	)
}

func (b *Builder) addUserDefinedTransformProcessor() buildComponentFunc {
	return b.AddProcessor(
		formatUserDefinedTransformProcessorID,
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if len(mp.Spec.Transforms) == 0 {
				return nil // No transforms, no processor needed
			}

			transformStatements := common.TransformSpecsToProcessorStatements(mp.Spec.Transforms)

			return common.MetricTransformProcessorConfig(transformStatements)
		},
	)
}

func (b *Builder) addUserDefinedFilterProcessor() buildComponentFunc {
	return b.AddProcessor(
		formatUserDefinedFilterProcessorID,
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if mp.Spec.Filters == nil {
				return nil // No filters, no processor needed
			}

			return common.FilterSpecsToMetricFilterProcessorConfig(mp.Spec.Filters)
		},
	)
}

// Batch processor

//nolint:mnd // hardcoded values
func (b *Builder) addBatchProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDBatchProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			return &common.BatchProcessor{
				SendBatchSize:    1024,
				Timeout:          "10s",
				SendBatchMaxSize: 1024,
			}
		},
	)
}

func (b *Builder) addOTLPExporter(queueSize int) buildComponentFunc {
	return b.AddExporter(
		formatOTLPExporterID,
		func(ctx context.Context, mp *telemetryv1beta1.MetricPipeline) (any, common.EnvVars, error) {
			otlpExporterBuilder := common.NewOTLPExporterConfigBuilder(
				b.Reader,
				mp.Spec.Output.OTLP,
				mp.Name,
				queueSize,
				common.SignalTypeMetric,
			)

			return otlpExporterBuilder.OTLPExporterConfig(ctx)
		},
	)
}

// Authentication extensions

func (b *Builder) addOAuth2Extension(ctx context.Context, pipeline *telemetryv1beta1.MetricPipeline) error {
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

// Helper functions

func shouldFilterByNamespace(namespaceSelector *telemetryv1beta1.NamespaceSelector) bool {
	return namespaceSelector != nil && (len(namespaceSelector.Include) > 0 || len(namespaceSelector.Exclude) > 0)
}

func filterByNamespaceProcessorConfig(namespaceSelector *telemetryv1beta1.NamespaceSelector) *common.FilterProcessor {
	var filterExpressions []string

	notFromKymaStatsReceiver := common.Not(common.KymaInputNameEquals(common.InputSourceKyma))

	if len(namespaceSelector.Exclude) > 0 {
		namespacesConditions := namespacesConditionsBuilder(namespaceSelector.Exclude)
		excludeNamespacesExpr := common.JoinWithAnd(notFromKymaStatsReceiver, common.JoinWithOr(namespacesConditions...))
		filterExpressions = append(filterExpressions, excludeNamespacesExpr)
	}

	if len(namespaceSelector.Include) > 0 {
		namespacesConditions := namespacesConditionsBuilder(namespaceSelector.Include)
		includeNamespacesExpr := common.JoinWithAnd(
			notFromKymaStatsReceiver,
			common.ResourceAttributeIsNotNil(common.K8sNamespaceName),
			common.Not(common.JoinWithOr(namespacesConditions...)),
		)
		filterExpressions = append(filterExpressions, includeNamespacesExpr)
	}

	return common.MetricFilterProcessorConfig(common.FilterProcessorMetrics{
		Metric: filterExpressions,
	})
}

func namespacesConditionsBuilder(namespaces []string) []string {
	var conditions []string
	for _, ns := range namespaces {
		conditions = append(conditions, common.NamespaceEquals(ns))
	}

	return conditions
}

func formatOTLPNamespaceFilterID(pipelineName string) string {
	return fmt.Sprintf(common.ComponentIDNamespacePerInputFilterProcessor, pipelineName, common.InputSourceOTLP)
}

func formatOTLPExporterID(pipeline *telemetryv1beta1.MetricPipeline) string {
	return common.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
}

func formatUserDefinedTransformProcessorID(mp *telemetryv1beta1.MetricPipeline) string {
	return fmt.Sprintf(common.ComponentIDUserDefinedTransformProcessor, mp.Name)
}

func formatUserDefinedFilterProcessorID(mp *telemetryv1beta1.MetricPipeline) string {
	return fmt.Sprintf(common.ComponentIDUserDefinedFilterProcessor, mp.Name)
}

func formatOutputServicePipelineID(mp *telemetryv1beta1.MetricPipeline) string {
	return fmt.Sprintf("metrics/%s-output", mp.Name)
}

func shouldEnableOAuth2(tp *telemetryv1beta1.MetricPipeline) bool {
	return tp.Spec.Output.OTLP.Authentication != nil && tp.Spec.Output.OTLP.Authentication.OAuth2 != nil
}
