package metricgateway

import (
	"context"
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	metricpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/metricpipeline"
)

var diagnosticMetricNames = []string{"up", "scrape_duration_seconds", "scrape_samples_scraped", "scrape_samples_post_metric_relabeling", "scrape_series_added"}

func (b *Builder) addOutputServicePipeline(ctx context.Context, mp *telemetryv1alpha1.MetricPipeline, fs ...buildComponentFunc) error {
	// Add an empty pipeline to the config
	pipelineID := formatOutputServicePipelineID(mp)
	b.config.Service.Pipelines[pipelineID] = common.Pipeline{}

	for _, f := range fs {
		if err := f(ctx, mp); err != nil {
			return fmt.Errorf("failed to add component: %w", err)
		}
	}

	return nil
}

func (b *Builder) addOutputReceiver(componentIDFunc componentIDFunc, configFunc componentConfigFunc) buildComponentFunc {
	return common.AddReceiver(b.config, componentIDFunc, configFunc, formatOutputServicePipelineID)
}

func (b *Builder) addOutputProcessor(componentIDFunc componentIDFunc, configFunc componentConfigFunc) buildComponentFunc {
	return common.AddProcessor(b.config, componentIDFunc, configFunc, formatOutputServicePipelineID)
}

func (b *Builder) addOutputExporter(componentIDFunc componentIDFunc, configFunc exporterComponentConfigFunc) buildComponentFunc {
	return common.AddExporter(b.config, b.envVars, componentIDFunc, configFunc, formatOutputServicePipelineID)
}

func (b *Builder) addOutputForwardReceiver() buildComponentFunc {
	return b.addOutputReceiver(
		formatForwardConnectorID,
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return &common.ForwardConnector{}
		},
	)
}

func (b *Builder) addOutputRoutingReceiver() buildComponentFunc {
	return b.addOutputReceiver(
		formatRoutingConnectorID,
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return enrichmentRoutingConnectorConfig(mp)
		},
	)
}

func (b *Builder) addSetInstrumentationScopeToKymaProcessor(opts BuildOptions) buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID(common.ComponentIDSetInstrumentationScopeKymaProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.InstrumentationScopeProcessorConfig(opts.InstrumentationScopeVersion, common.InputSourceKyma)
		},
	)
}

// Input source filter processors

func (b *Builder) addDropIfRuntimeInputDisabledProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID(common.ComponentIDDropIfInputSourceRuntimeProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{common.ScopeNameEquals(common.InstrumentationScopeRuntime)},
				},
			}
		},
	)
}

func (b *Builder) addDropIfPrometheusInputDisabledProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID(common.ComponentIDDropIfInputSourcePrometheusProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsPrometheusInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{common.ResourceAttributeEquals(common.KymaInputNameAttribute, common.KymaInputPrometheus)},
				},
			}
		},
	)
}

func (b *Builder) addDropIfIstioInputDisabledProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID(common.ComponentIDDropIfInputSourceIstioProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsIstioInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{common.ScopeNameEquals(common.InstrumentationScopeIstio)},
				},
			}
		},
	)
}

func (b *Builder) addDropIfOTLPInputDisabledProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID(common.ComponentIDDropIfInputSourceOTLPProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsOTLPInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{ottlUknownInputSource()},
				},
			}
		},
	)
}

func (b *Builder) addDropEnvoyMetricsIfDisabledProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID(common.ComponentIDDropEnvoyMetricsIfDisabledProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsIstioInputEnabled(mp.Spec.Input) && metricpipelineutils.IsEnvoyMetricsEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{
						common.JoinWithAnd(common.IsMatch("name", "^envoy_.*"), common.ScopeNameEquals(common.InstrumentationScopeIstio)),
					},
				},
			}
		},
	)
}

// Namespace filter processors

func (b *Builder) addRuntimeNamespaceFilterProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		func(mp *telemetryv1alpha1.MetricPipeline) string {
			return formatNamespaceFilterID(mp.Name, common.InputSourceRuntime)
		},
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			input := mp.Spec.Input
			if !metricpipelineutils.IsRuntimeInputEnabled(input) || !shouldFilterByNamespace(input.Runtime.Namespaces) {
				return nil
			}

			return filterByNamespaceProcessorConfig(input.Runtime.Namespaces, inputSourceEquals(common.InputSourceRuntime))
		},
	)
}

func (b *Builder) addPrometheusNamespaceFilterProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		func(mp *telemetryv1alpha1.MetricPipeline) string {
			return formatNamespaceFilterID(mp.Name, common.InputSourcePrometheus)
		},
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			input := mp.Spec.Input
			if !metricpipelineutils.IsPrometheusInputEnabled(input) || !shouldFilterByNamespace(input.Prometheus.Namespaces) {
				return nil
			}

			return filterByNamespaceProcessorConfig(input.Prometheus.Namespaces, common.ResourceAttributeEquals(common.KymaInputNameAttribute, common.KymaInputPrometheus))
		},
	)
}

func (b *Builder) addIstioNamespaceFilterProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		func(mp *telemetryv1alpha1.MetricPipeline) string {
			return formatNamespaceFilterID(mp.Name, common.InputSourceIstio)
		},
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			input := mp.Spec.Input
			if !metricpipelineutils.IsIstioInputEnabled(input) || !shouldFilterByNamespace(input.Istio.Namespaces) {
				return nil
			}

			return filterByNamespaceProcessorConfig(input.Istio.Namespaces, inputSourceEquals(common.InputSourceIstio))
		},
	)
}

func (b *Builder) addOTLPNamespaceFilterProcessor() buildComponentFunc {
	return b.addOutputProcessor(
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

// Runtime resource filter processors

func (b *Builder) addDropRuntimePodMetricsProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID(common.ComponentIDDropRuntimePodMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimePodInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{
						common.JoinWithAnd(inputSourceEquals(common.InputSourceRuntime), common.IsMatch("name", "^k8s.pod.*")),
					},
				},
			}
		},
	)
}

func (b *Builder) addDropRuntimeContainerMetricsProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID(common.ComponentIDDropRuntimeContainerMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeContainerInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{
						common.JoinWithAnd(inputSourceEquals(common.InputSourceRuntime), common.IsMatch("name", "(^k8s.container.*)|(^container.*)")),
					},
				},
			}
		},
	)
}

func (b *Builder) addDropRuntimeNodeMetricsProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID(common.ComponentIDDropRuntimeNodeMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeNodeInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{
						common.JoinWithAnd(inputSourceEquals(common.InputSourceRuntime), common.IsMatch("name", "^k8s.node.*")),
					},
				},
			}
		},
	)
}

func (b *Builder) addDropRuntimeVolumeMetricsProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID(common.ComponentIDDropRuntimeVolumeMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeVolumeInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{
						common.JoinWithAnd(inputSourceEquals(common.InputSourceRuntime), common.IsMatch("name", "^k8s.volume.*")),
					},
				},
			}
		},
	)
}

func (b *Builder) addDropRuntimeDeploymentMetricsProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID(common.ComponentIDDropRuntimeDeploymentMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeDeploymentInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{
						common.JoinWithAnd(inputSourceEquals(common.InputSourceRuntime), common.IsMatch("name", "^k8s.deployment.*")),
					},
				},
			}
		},
	)
}

func (b *Builder) addDropRuntimeDaemonSetMetricsProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID(common.ComponentIDDropRuntimeDaemonSetMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeDaemonSetInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{
						common.JoinWithAnd(inputSourceEquals(common.InputSourceRuntime), common.IsMatch("name", "^k8s.daemonset.*")),
					},
				},
			}
		},
	)
}

func (b *Builder) addDropRuntimeStatefulSetMetricsProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID(common.ComponentIDDropRuntimeStatefulSetMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeStatefulSetInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{
						common.JoinWithAnd(inputSourceEquals(common.InputSourceRuntime), common.IsMatch("name", "^k8s.statefulset.*")),
					},
				},
			}
		},
	)
}

func (b *Builder) addDropRuntimeJobMetricsProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID(common.ComponentIDDropRuntimeJobMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeJobInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{
						common.JoinWithAnd(inputSourceEquals(common.InputSourceRuntime), common.IsMatch("name", "^k8s.job.*")),
					},
				},
			}
		},
	)
}

// Diagnostic metric filter processors

func (b *Builder) addDropPrometheusDiagnosticMetricsProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID(common.ComponentIDDropPrometheusDiagnosticMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsPrometheusInputEnabled(mp.Spec.Input) || metricpipelineutils.IsPrometheusDiagnosticInputEnabled(mp.Spec.Input) {
				return nil
			}

			return dropDiagnosticMetricsFilterConfig(inputSourceEquals(common.InputSourcePrometheus))
		},
	)
}

func (b *Builder) addDropIstioDiagnosticMetricsProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID(common.ComponentIDDropIstioDiagnosticMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsIstioInputEnabled(mp.Spec.Input) || metricpipelineutils.IsIstioDiagnosticInputEnabled(mp.Spec.Input) {
				return nil
			}

			return dropDiagnosticMetricsFilterConfig(inputSourceEquals(common.InputSourceIstio))
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

func filterByNamespaceProcessorConfig(namespaceSelector *telemetryv1alpha1.NamespaceSelector, inputSourceCondition string) *FilterProcessor {
	var filterExpressions []string

	if len(namespaceSelector.Exclude) > 0 {
		namespacesConditions := namespacesConditions(namespaceSelector.Exclude)
		excludeNamespacesExpr := common.JoinWithAnd(inputSourceCondition, common.JoinWithOr(namespacesConditions...))
		filterExpressions = append(filterExpressions, excludeNamespacesExpr)
	}

	if len(namespaceSelector.Include) > 0 {
		namespacesConditions := namespacesConditions(namespaceSelector.Include)
		includeNamespacesExpr := common.JoinWithAnd(
			inputSourceCondition,
			common.ResourceAttributeIsNotNil(common.K8sNamespaceName),
			common.Not(common.JoinWithOr(namespacesConditions...)),
		)
		filterExpressions = append(filterExpressions, includeNamespacesExpr)
	}

	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: filterExpressions,
		},
	}
}

func namespacesConditions(namespaces []string) []string {
	var conditions []string
	for _, ns := range namespaces {
		conditions = append(conditions, common.NamespaceEquals(ns))
	}

	return conditions
}

// Resource processors

func (b *Builder) addInsertClusterAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID(common.ComponentIDInsertClusterAttributesProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.InsertClusterAttributesProcessorConfig(opts.ClusterName, opts.ClusterUID, opts.CloudProvider)
		},
	)
}

func (b *Builder) addDeleteSkipEnrichmentAttributeProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID(common.ComponentIDDeleteSkipEnrichmentAttributeProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return &common.ResourceProcessor{
				Attributes: []common.AttributeAction{
					{
						Action: "delete",
						Key:    common.SkipEnrichmentAttribute,
					},
				},
			}
		},
	)
}

func (b *Builder) addDropKymaAttributesProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID(common.ComponentIDDropKymaAttributesProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.DropKymaAttributesProcessorConfig()
		},
	)
}

func (b *Builder) addUserDefinedTransformProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		func(mp *telemetryv1alpha1.MetricPipeline) string {
			return fmt.Sprintf("transform/%s-user-defined", mp.Name)
		},
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

// Batch processor

//nolint:mnd // hardcoded values
func (b *Builder) addBatchProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID(common.ComponentIDBatchProcessor),
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
	return b.addOutputExporter(
		formatOTLPExporterID,
		func(ctx context.Context, mp *telemetryv1alpha1.MetricPipeline) (any, common.EnvVars, error) {
			otlpExporterBuilder := common.NewOTLPExporterConfigBuilder(
				b.Reader,
				mp.Spec.Output.OTLP,
				mp.Name,
				queueSize,
				common.SignalTypeLog,
			)

			return otlpExporterBuilder.OTLPExporterConfig(ctx)
		},
	)
}

func dropDiagnosticMetricsFilterConfig(inputSourceCondition string) *FilterProcessor {
	var filterExpressions []string

	metricNameConditions := nameConditions(diagnosticMetricNames)
	excludeScrapeMetricsExpr := common.JoinWithAnd(inputSourceCondition, common.JoinWithOr(metricNameConditions...))
	filterExpressions = append(filterExpressions, excludeScrapeMetricsExpr)

	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: filterExpressions,
		},
	}
}

func nameConditions(names []string) []string {
	var nameConditions []string
	for _, name := range names {
		nameConditions = append(nameConditions, common.NameAttributeEquals(name))
	}

	return nameConditions
}

func formatNamespaceFilterID(pipelineName string, inputSourceType common.InputSourceType) string {
	return fmt.Sprintf(common.ComponentIDNamespacePerInputFilterProcessor, pipelineName, inputSourceType)
}

func formatOTLPExporterID(pipeline *telemetryv1alpha1.MetricPipeline) string {
	return common.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
}
