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
			return common.SkipEnrichmentRoutingConnectorConfig(
				[]string{formatEnrichmentServicePipelineID(mp)},
				[]string{formatOutputServicePipelineID(mp)},
			)
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

func (b *Builder) addDropIfRuntimeInputDisabledProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropIfInputSourceRuntimeProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &common.FilterProcessor{
				Metrics: common.FilterProcessorMetrics{
					Metric: []string{common.ScopeNameEquals(common.InstrumentationScopeRuntime)},
				},
			}
		},
	)
}

func (b *Builder) addDropIfPrometheusInputDisabledProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropIfInputSourcePrometheusProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsPrometheusInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &common.FilterProcessor{
				Metrics: common.FilterProcessorMetrics{
					Metric: []string{common.ResourceAttributeEquals(common.KymaInputNameAttribute, common.KymaInputPrometheus)},
				},
			}
		},
	)
}

func (b *Builder) addDropIfIstioInputDisabledProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropIfInputSourceIstioProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsIstioInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &common.FilterProcessor{
				Metrics: common.FilterProcessorMetrics{
					Metric: []string{common.ScopeNameEquals(common.InstrumentationScopeIstio)},
				},
			}
		},
	)
}

func (b *Builder) addDropIfOTLPInputDisabledProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropIfInputSourceOTLPProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsOTLPInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &common.FilterProcessor{
				Metrics: common.FilterProcessorMetrics{
					Metric: []string{ottlUknownInputSource()},
				},
			}
		},
	)
}

func (b *Builder) addDropEnvoyMetricsIfDisabledProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropEnvoyMetricsIfDisabledProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsIstioInputEnabled(mp.Spec.Input) && metricpipelineutils.IsEnvoyMetricsEnabled(mp.Spec.Input) {
				return nil
			}

			return &common.FilterProcessor{
				Metrics: common.FilterProcessorMetrics{
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
	return b.AddProcessor(
		func(mp *telemetryv1alpha1.MetricPipeline) string {
			return formatNamespaceFilterID(mp.Name, common.InputSourceRuntime)
		},
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			input := mp.Spec.Input
			if !metricpipelineutils.IsRuntimeInputEnabled(input) || !shouldFilterByNamespace(input.Runtime.Namespaces) {
				return nil
			}

			return common.FilterByNamespaceProcessorConfig(input.Runtime.Namespaces, common.InputSourceEquals(common.InputSourceRuntime))
		},
	)
}

func (b *Builder) addPrometheusNamespaceFilterProcessor() buildComponentFunc {
	return b.AddProcessor(
		func(mp *telemetryv1alpha1.MetricPipeline) string {
			return formatNamespaceFilterID(mp.Name, common.InputSourcePrometheus)
		},
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			input := mp.Spec.Input
			if !metricpipelineutils.IsPrometheusInputEnabled(input) || !shouldFilterByNamespace(input.Prometheus.Namespaces) {
				return nil
			}

			return common.FilterByNamespaceProcessorConfig(input.Prometheus.Namespaces, common.ResourceAttributeEquals(common.KymaInputNameAttribute, common.KymaInputPrometheus))
		},
	)
}

func (b *Builder) addIstioNamespaceFilterProcessor() buildComponentFunc {
	return b.AddProcessor(
		func(mp *telemetryv1alpha1.MetricPipeline) string {
			return formatNamespaceFilterID(mp.Name, common.InputSourceIstio)
		},
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			input := mp.Spec.Input
			if !metricpipelineutils.IsIstioInputEnabled(input) || !shouldFilterByNamespace(input.Istio.Namespaces) {
				return nil
			}

			return common.FilterByNamespaceProcessorConfig(input.Istio.Namespaces, common.InputSourceEquals(common.InputSourceIstio))
		},
	)
}

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

			return common.FilterByNamespaceProcessorConfig(input.OTLP.Namespaces, ottlUknownInputSource())
		},
	)
}

// Runtime resource filter processors

func (b *Builder) addDropRuntimePodMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropRuntimePodMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimePodInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &common.FilterProcessor{
				Metrics: common.FilterProcessorMetrics{
					Metric: []string{
						common.JoinWithAnd(common.InputSourceEquals(common.InputSourceRuntime), common.IsMatch("name", "^k8s.pod.*")),
					},
				},
			}
		},
	)
}

func (b *Builder) addDropRuntimeContainerMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropRuntimeContainerMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeContainerInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &common.FilterProcessor{
				Metrics: common.FilterProcessorMetrics{
					Metric: []string{
						common.JoinWithAnd(common.InputSourceEquals(common.InputSourceRuntime), common.IsMatch("name", "(^k8s.container.*)|(^container.*)")),
					},
				},
			}
		},
	)
}

func (b *Builder) addDropRuntimeNodeMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropRuntimeNodeMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeNodeInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &common.FilterProcessor{
				Metrics: common.FilterProcessorMetrics{
					Metric: []string{
						common.JoinWithAnd(common.InputSourceEquals(common.InputSourceRuntime), common.IsMatch("name", "^k8s.node.*")),
					},
				},
			}
		},
	)
}

func (b *Builder) addDropRuntimeVolumeMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropRuntimeVolumeMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeVolumeInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &common.FilterProcessor{
				Metrics: common.FilterProcessorMetrics{
					Metric: []string{
						common.JoinWithAnd(common.InputSourceEquals(common.InputSourceRuntime), common.IsMatch("name", "^k8s.volume.*")),
					},
				},
			}
		},
	)
}

func (b *Builder) addDropRuntimeDeploymentMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropRuntimeDeploymentMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeDeploymentInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &common.FilterProcessor{
				Metrics: common.FilterProcessorMetrics{
					Metric: []string{
						common.JoinWithAnd(common.InputSourceEquals(common.InputSourceRuntime), common.IsMatch("name", "^k8s.deployment.*")),
					},
				},
			}
		},
	)
}

func (b *Builder) addDropRuntimeDaemonSetMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropRuntimeDaemonSetMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeDaemonSetInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &common.FilterProcessor{
				Metrics: common.FilterProcessorMetrics{
					Metric: []string{
						common.JoinWithAnd(common.InputSourceEquals(common.InputSourceRuntime), common.IsMatch("name", "^k8s.daemonset.*")),
					},
				},
			}
		},
	)
}

func (b *Builder) addDropRuntimeStatefulSetMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropRuntimeStatefulSetMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeStatefulSetInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &common.FilterProcessor{
				Metrics: common.FilterProcessorMetrics{
					Metric: []string{
						common.JoinWithAnd(common.InputSourceEquals(common.InputSourceRuntime), common.IsMatch("name", "^k8s.statefulset.*")),
					},
				},
			}
		},
	)
}

func (b *Builder) addDropRuntimeJobMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropRuntimeJobMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeJobInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &common.FilterProcessor{
				Metrics: common.FilterProcessorMetrics{
					Metric: []string{
						common.JoinWithAnd(common.InputSourceEquals(common.InputSourceRuntime), common.IsMatch("name", "^k8s.job.*")),
					},
				},
			}
		},
	)
}

// Diagnostic metric filter processors

func (b *Builder) addDropPrometheusDiagnosticMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropPrometheusDiagnosticMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsPrometheusInputEnabled(mp.Spec.Input) || metricpipelineutils.IsPrometheusDiagnosticInputEnabled(mp.Spec.Input) {
				return nil
			}

			return common.DropDiagnosticMetricsFilterProcessorConfig(common.InputSourcePrometheus)
		},
	)
}

func (b *Builder) addDropIstioDiagnosticMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropIstioDiagnosticMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsIstioInputEnabled(mp.Spec.Input) || metricpipelineutils.IsIstioDiagnosticInputEnabled(mp.Spec.Input) {
				return nil
			}

			return common.DropDiagnosticMetricsFilterProcessorConfig(common.InputSourceIstio)
		},
	)
}

// Helper functions

func shouldFilterByNamespace(namespaceSelector *telemetryv1alpha1.NamespaceSelector) bool {
	return namespaceSelector != nil && (len(namespaceSelector.Include) > 0 || len(namespaceSelector.Exclude) > 0)
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

// Resource processors

func (b *Builder) addInsertClusterAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDInsertClusterAttributesProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.InsertClusterAttributesProcessorConfig(opts.ClusterName, opts.ClusterUID, opts.CloudProvider)
		},
	)
}

func (b *Builder) addDeleteSkipEnrichmentAttributeProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDeleteSkipEnrichmentAttributeProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.DeleteSkipEnrichmentAttributeProcessorConfig()
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
				common.SignalTypeLog,
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
