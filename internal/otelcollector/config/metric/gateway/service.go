package gateway

import (
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
	metricpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/metricpipeline"
)

// Service pipeline assembly

func (b *Builder) addServicePipelines(pipeline *telemetryv1alpha1.MetricPipeline) {
	inputPipelineID := formatMetricInputPipelineID(pipeline.Name)
	enrichmentPipelineID := formatMetricEnrichmentPipelineID(pipeline.Name)
	outputPipelineID := formatMetricOutputPipelineID(pipeline.Name)

	b.config.Service.Pipelines[inputPipelineID] = inputPipelineConfig(pipeline)
	b.config.Service.Pipelines[enrichmentPipelineID] = enrichmentPipelineConfig(pipeline.Name)
	b.config.Service.Pipelines[outputPipelineID] = outputPipelineConfig(pipeline)
}

// Pipeline ID formatting functions

func formatMetricInputPipelineID(pipelineName string) string {
	return fmt.Sprintf("metrics/%s-input", pipelineName)
}

func formatMetricEnrichmentPipelineID(pipelineName string) string {
	return fmt.Sprintf("metrics/%s-attributes-enrichment", pipelineName)
}

func formatMetricOutputPipelineID(pipelineName string) string {
	return fmt.Sprintf("metrics/%s-output", pipelineName)
}

// Pipeline configuration functions

func inputPipelineConfig(pipeline *telemetryv1alpha1.MetricPipeline) config.Pipeline {
	return config.Pipeline{
		Receivers:  []string{"otlp", "kymastats"},
		Processors: []string{"memory_limiter"},
		Exporters:  []string{formatRoutingConnectorID(pipeline.Name)},
	}
}

func enrichmentPipelineConfig(pipelineName string) config.Pipeline {
	return config.Pipeline{
		Receivers:  []string{formatRoutingConnectorID(pipelineName)},
		Processors: []string{"k8sattributes", "service_enrichment"},
		Exporters:  []string{formatForwardConnectorID(pipelineName)},
	}
}

func outputPipelineConfig(pipeline *telemetryv1alpha1.MetricPipeline) config.Pipeline {
	var processors []string

	input := pipeline.Spec.Input

	processors = append(processors, "transform/set-instrumentation-scope-kyma")
	processors = append(processors, inputSourceFiltersIDs(input)...)
	processors = append(processors, namespaceFiltersIDs(input, pipeline)...)
	processors = append(processors, runtimeResourcesFiltersIDs(input)...)
	processors = append(processors, diagnosticMetricFiltersIDs(input)...)

	// Add transform processors if transforms are specified
	if len(pipeline.Spec.Transforms) > 0 {
		processors = append(processors, formatTransformProcessorID(pipeline.Name))
	}

	processors = append(processors, "resource/insert-cluster-attributes", "resource/delete-skip-enrichment-attribute", "resource/drop-kyma-attributes", "batch")

	return config.Pipeline{
		Receivers:  []string{formatRoutingConnectorID(pipeline.Name), formatForwardConnectorID(pipeline.Name)},
		Processors: processors,
		Exporters:  []string{formatOTLPExporterID(pipeline)},
	}
}

func inputSourceFiltersIDs(input telemetryv1alpha1.MetricPipelineInput) []string {
	var processors []string

	if !metricpipelineutils.IsRuntimeInputEnabled(input) {
		processors = append(processors, "filter/drop-if-input-source-runtime")
	}

	if !metricpipelineutils.IsPrometheusInputEnabled(input) {
		processors = append(processors, "filter/drop-if-input-source-prometheus")
	}

	if !metricpipelineutils.IsIstioInputEnabled(input) {
		processors = append(processors, "filter/drop-if-input-source-istio")
	}

	if !metricpipelineutils.IsIstioInputEnabled(input) || !metricpipelineutils.IsEnvoyMetricsEnabled(input) {
		processors = append(processors, "filter/drop-envoy-metrics-if-disabled")
	}

	if !metricpipelineutils.IsOTLPInputEnabled(input) {
		processors = append(processors, "filter/drop-if-input-source-otlp")
	}

	return processors
}

func namespaceFiltersIDs(input telemetryv1alpha1.MetricPipelineInput, pipeline *telemetryv1alpha1.MetricPipeline) []string {
	var processors []string

	if metricpipelineutils.IsRuntimeInputEnabled(input) && shouldFilterByNamespace(input.Runtime.Namespaces) {
		processors = append(processors, formatNamespaceFilterID(pipeline.Name, metric.InputSourceRuntime))
	}

	if metricpipelineutils.IsPrometheusInputEnabled(input) && shouldFilterByNamespace(input.Prometheus.Namespaces) {
		processors = append(processors, formatNamespaceFilterID(pipeline.Name, metric.InputSourcePrometheus))
	}

	if metricpipelineutils.IsIstioInputEnabled(input) && shouldFilterByNamespace(input.Istio.Namespaces) {
		processors = append(processors, formatNamespaceFilterID(pipeline.Name, metric.InputSourceIstio))
	}

	if metricpipelineutils.IsOTLPInputEnabled(input) && input.OTLP != nil && shouldFilterByNamespace(input.OTLP.Namespaces) {
		processors = append(processors, formatNamespaceFilterID(pipeline.Name, metric.InputSourceOTLP))
	}

	return processors
}

func runtimeResourcesFiltersIDs(input telemetryv1alpha1.MetricPipelineInput) []string {
	var processors []string

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimePodInputEnabled(input) {
		processors = append(processors, "filter/drop-runtime-pod-metrics")
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeContainerInputEnabled(input) {
		processors = append(processors, "filter/drop-runtime-container-metrics")
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeNodeInputEnabled(input) {
		processors = append(processors, "filter/drop-runtime-node-metrics")
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeVolumeInputEnabled(input) {
		processors = append(processors, "filter/drop-runtime-volume-metrics")
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeDeploymentInputEnabled(input) {
		processors = append(processors, "filter/drop-runtime-deployment-metrics")
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeDaemonSetInputEnabled(input) {
		processors = append(processors, "filter/drop-runtime-daemonset-metrics")
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeStatefulSetInputEnabled(input) {
		processors = append(processors, "filter/drop-runtime-statefulset-metrics")
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeJobInputEnabled(input) {
		processors = append(processors, "filter/drop-runtime-job-metrics")
	}

	return processors
}

func diagnosticMetricFiltersIDs(input telemetryv1alpha1.MetricPipelineInput) []string {
	var processors []string

	if metricpipelineutils.IsIstioInputEnabled(input) && !metricpipelineutils.IsIstioDiagnosticInputEnabled(input) {
		processors = append(processors, "filter/drop-diagnostic-metrics-if-input-source-istio")
	}

	if metricpipelineutils.IsPrometheusInputEnabled(input) && !metricpipelineutils.IsPrometheusDiagnosticInputEnabled(input) {
		processors = append(processors, "filter/drop-diagnostic-metrics-if-input-source-prometheus")
	}

	return processors
}

func formatNamespaceFilterID(pipelineName string, inputSourceType metric.InputSourceType) string {
	return fmt.Sprintf("filter/%s-filter-by-namespace-%s-input", pipelineName, inputSourceType)
}

func formatForwardConnectorID(pipelineName string) string {
	return fmt.Sprintf("forward/%s", pipelineName)
}

func formatRoutingConnectorID(pipelineName string) string {
	return fmt.Sprintf("routing/%s", pipelineName)
}

func formatTransformProcessorID(pipelineName string) string {
	return fmt.Sprintf("transform/%s", pipelineName)
}

// Helper functions

func formatOTLPExporterID(pipeline *telemetryv1alpha1.MetricPipeline) string {
	return otlpexporter.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
}
