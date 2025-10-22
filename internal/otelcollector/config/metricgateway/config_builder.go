package metricgateway

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	metricpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/metricpipeline"
)

type buildComponentFunc = common.BuildComponentFunc[*telemetryv1alpha1.MetricPipeline]

type Builder struct {
	common.ComponentBuilder[*telemetryv1alpha1.MetricPipeline]

	Reader client.Reader
}

type BuildOptions struct {
	GatewayNamespace            string
	InstrumentationScopeVersion string
	ClusterName                 string
	ClusterUID                  string
	CloudProvider               string
	Enrichments                 *operatorv1alpha1.EnrichmentSpec
}

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.MetricPipeline, opts BuildOptions) (*common.Config, common.EnvVars, error) {
	b.Config = common.NewConfig()
	b.AddExtension(common.ComponentIDK8sLeaderElectorExtension,
		common.K8sLeaderElector{
			AuthType:       "serviceAccount",
			LeaseName:      common.K8sLeaderElectorKymaStats,
			LeaseNamespace: opts.GatewayNamespace,
		},
	)
	b.EnvVars = make(common.EnvVars)

	queueSize := common.BatchingMaxQueueSize / len(pipelines)

	if err := b.AddServicePipeline(ctx, nil, "metrics/input",
		b.addOTLPReceiver(),
		b.addKymaStatsReceiver(),
		b.addMemoryLimiterProcessor(),
		b.addSetInstrumentationScopeToKymaProcessor(opts),
		b.addK8sAttributesProcessor(opts),
		b.addServiceEnrichmentProcessor(),
		b.addInsertClusterAttributesProcessor(opts),
		// Kyma attributes are dropped before user-defined transform and filter processors
		// to prevent user access to internal attributes.
		b.addDropKymaAttributesProcessor(),
		b.addInputRoutingExporter(),
	); err != nil {
		return nil, nil, fmt.Errorf("failed to add input service pipeline: %w", err)
	}

	for _, pipeline := range pipelines {
		outputPipelineID := formatOutputServicePipelineID(&pipeline)
		if err := b.AddServicePipeline(ctx, &pipeline, outputPipelineID,
			b.addOutputForwardReceiver(),
			// Input source filters if otlp is disabled
			b.addDropIfOTLPInputDisabledProcessor(),
			// Namespace filters
			b.addOTLPNamespaceFilterProcessor(),
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
		func(mp *telemetryv1alpha1.MetricPipeline) any {
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
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return &KymaStatsReceiver{
				AuthType:           "serviceAccount",
				K8sLeaderElector:   "k8s_leader_elector",
				CollectionInterval: "30s",
				Resources: []ModuleGVR{
					{Group: "operator.kyma-project.io", Version: "v1alpha1", Resource: "telemetries"},
					{Group: "telemetry.kyma-project.io", Version: "v1alpha1", Resource: "logpipelines"},
					{Group: "telemetry.kyma-project.io", Version: "v1alpha1", Resource: "tracepipelines"},
					{Group: "telemetry.kyma-project.io", Version: "v1alpha1", Resource: "metricpipelines"},
				},
			}
		},
	)
}

//nolint:mnd // hardcoded values
func (b *Builder) addMemoryLimiterProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDMemoryLimiterProcessor),
		func(lp *telemetryv1alpha1.MetricPipeline) any {
			return &common.MemoryLimiter{
				CheckInterval:        "1s",
				LimitPercentage:      75,
				SpikeLimitPercentage: 15,
			}
		},
	)
}

func (b *Builder) addInputRoutingExporter() buildComponentFunc {
	return b.AddExporter(
		b.StaticComponentID(common.ComponentIDForwardConnector),
		func(ctx context.Context, mp *telemetryv1alpha1.MetricPipeline) (any, common.EnvVars, error) {
			return &common.ForwardConnector{}, nil, nil
		},
	)
}

func (b *Builder) addOutputForwardReceiver() buildComponentFunc {
	return b.AddReceiver(
		b.StaticComponentID(common.ComponentIDForwardConnector),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return &common.ForwardConnector{}
		},
	)
}

func (b *Builder) addSetInstrumentationScopeToKymaProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDSetInstrumentationScopeKymaProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.InstrumentationScopeProcessorConfig(opts.InstrumentationScopeVersion, common.InputSourceKyma)
		},
	)
}

// ======================================================
// Output pipeline components
// ======================================================

// Input source filter processors

func (b *Builder) addDropIfOTLPInputDisabledProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropIfInputSourceOTLPProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsOTLPInputEnabled(mp.Spec.Input) {
				return nil
			}

			return common.MetricFilterProcessorConfig(common.FilterProcessorMetrics{
				Metric: []string{common.ScopeNameEquals(fmt.Sprintf("not(%s)", common.InstrumentationScopeKyma))},
			})
		},
	)
}

// Namespace filter processors

func (b *Builder) addOTLPNamespaceFilterProcessor() buildComponentFunc {
	return b.AddProcessor(
		func(mp *telemetryv1alpha1.MetricPipeline) string {
			return formatNamespaceFilterID(mp.Name, common.InputSourceOTLP)
		},
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			input := mp.Spec.Input
			if !metricpipelineutils.IsOTLPInputEnabled(input) || input.OTLP == nil || !shouldFilterByNamespace(input.OTLP.Namespaces) {
				return nil
			}

			return filterByNamespaceProcessorConfig(input.OTLP.Namespaces, fmt.Sprintf("not(%s)", common.ScopeNameEquals(common.InstrumentationScopeKyma)))
		},
	)
}

// Resource processors

func (b *Builder) addInsertClusterAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDInsertClusterAttributesProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.InsertClusterAttributesProcessorConfig(opts.ClusterName, opts.ClusterUID, opts.CloudProvider)
		},
	)
}

func (b *Builder) addDropKymaAttributesProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropKymaAttributesProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.DropKymaAttributesProcessorConfig()
		},
	)
}

func (b *Builder) addUserDefinedTransformProcessor() buildComponentFunc {
	return b.AddProcessor(
		formatUserDefinedTransformProcessorID,
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if len(mp.Spec.Transforms) == 0 {
				return nil // No transforms, no processor needed
			}

			transformStatements := common.TransformSpecsToProcessorStatements(mp.Spec.Transforms)
			transformProcessor := common.MetricTransformProcessorConfig(transformStatements)

			return transformProcessor
		},
	)
}

func (b *Builder) addUserDefinedFilterProcessor() buildComponentFunc {
	return b.AddProcessor(
		formatUserDefinedFilterProcessorID,
		func(mp *telemetryv1alpha1.MetricPipeline) any {
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
		func(mp *telemetryv1alpha1.MetricPipeline) any {
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
		func(ctx context.Context, mp *telemetryv1alpha1.MetricPipeline) (any, common.EnvVars, error) {
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

// Helper functions

func shouldFilterByNamespace(namespaceSelector *telemetryv1alpha1.NamespaceSelector) bool {
	return namespaceSelector != nil && (len(namespaceSelector.Include) > 0 || len(namespaceSelector.Exclude) > 0)
}

func filterByNamespaceProcessorConfig(namespaceSelector *telemetryv1alpha1.NamespaceSelector, inputSourceCondition string) *common.FilterProcessor {
	var filterExpressions []string

	if len(namespaceSelector.Exclude) > 0 {
		namespacesConditions := namespacesConditionsBuilder(namespaceSelector.Exclude)
		excludeNamespacesExpr := common.JoinWithAnd(inputSourceCondition, common.JoinWithOr(namespacesConditions...))
		filterExpressions = append(filterExpressions, excludeNamespacesExpr)
	}

	if len(namespaceSelector.Include) > 0 {
		namespacesConditions := namespacesConditionsBuilder(namespaceSelector.Include)
		includeNamespacesExpr := common.JoinWithAnd(
			inputSourceCondition,
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

func formatNamespaceFilterID(pipelineName string, inputSourceType common.InputSourceType) string {
	return fmt.Sprintf(common.ComponentIDNamespacePerInputFilterProcessor, pipelineName, inputSourceType)
}

func formatOTLPExporterID(pipeline *telemetryv1alpha1.MetricPipeline) string {
	return common.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
}

func formatUserDefinedTransformProcessorID(mp *telemetryv1alpha1.MetricPipeline) string {
	return fmt.Sprintf(common.ComponentIDUserDefinedTransformProcessor, mp.Name)
}

func formatUserDefinedFilterProcessorID(mp *telemetryv1alpha1.MetricPipeline) string {
	return fmt.Sprintf(common.ComponentIDUserDefinedFilterProcessor, mp.Name)
}
func formatOutputServicePipelineID(mp *telemetryv1alpha1.MetricPipeline) string {
	return fmt.Sprintf("metrics/%s-output", mp.Name)
}
