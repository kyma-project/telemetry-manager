package metricgateway

import (
	"context"
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	metricpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/metricpipeline"
)

func (b *Builder) addOutputForwardReceiver() buildComponentFunc {
	return b.AddReceiver(
		formatForwardConnectorID,
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return &common.ForwardConnector{}
		},
	)
}

func (b *Builder) addOutputRoutingReceiver() buildComponentFunc {
	return b.AddReceiver(
		formatRoutingConnectorID,
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return enrichmentRoutingConnectorConfig(mp)
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

// Input source filter processors

func (b *Builder) addDropIfOTLPInputDisabledProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropIfInputSourceOTLPProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsOTLPInputEnabled(mp.Spec.Input) {
				return nil
			}

			return common.MetricFilterProcessorConfig(common.FilterProcessorMetrics{
				Metric: []string{ottlUknownInputSource()},
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

			return filterByNamespaceProcessorConfig(input.OTLP.Namespaces, ottlUknownInputSource())
		},
	)
}

// Helper functions

func shouldFilterByNamespace(namespaceSelector *telemetryv1alpha1.NamespaceSelector) bool {
	return namespaceSelector != nil && (len(namespaceSelector.Include) > 0 || len(namespaceSelector.Exclude) > 0)
}

func inputSourceEquals(inputSourceType common.InputSourceType) string {
	return common.ScopeNameEquals(common.InstrumentationScope[inputSourceType])
}

// When instrumentation scope is not set to any of the following values
// io.kyma-project.telemetry/runtime, io.kyma-project.telemetry/prometheus, io.kyma-project.telemetry/istio, and io.kyma-project.telemetry/kyma
// we assume the metric is being pushed directly to metrics gateway.
func ottlUknownInputSource() string {
	return fmt.Sprintf("not(%s or %s or %s or %s)",
		common.ScopeNameEquals(common.InstrumentationScopeRuntime),
		common.ResourceAttributeEquals(common.KymaInputNameAttribute, common.KymaInputPrometheus),
		common.ScopeNameEquals(common.InstrumentationScopeIstio),
		common.ScopeNameEquals(common.InstrumentationScopeKyma),
	)
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
