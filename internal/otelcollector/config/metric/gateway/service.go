package gateway

import (
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
)

func makeInputPipelineServiceConfig(pipeline *telemetryv1alpha1.MetricPipeline) config.Pipeline {
	return config.Pipeline{
		Receivers:  makeReceiversIDs(pipeline),
		Processors: []string{"memory_limiter"},
		Exporters:  []string{formatRoutingConnectorID(pipeline.Name)},
	}
}

func makeAttributesEnrichmentPipelineServiceConfig(pipelineName string) config.Pipeline {
	return config.Pipeline{
		Receivers:  []string{formatRoutingConnectorID(pipelineName)},
		Processors: []string{"k8sattributes", "transform/resolve-service-name", "resource/drop-kyma-attributes"},
		Exporters:  []string{formatForwardConnectorID(pipelineName)},
	}
}

func makeOutputPipelineServiceConfig(pipeline *telemetryv1alpha1.MetricPipeline) config.Pipeline {
	var processors []string

	input := pipeline.Spec.Input

	// Perform the transform before runtime resource filter as InstrumentationScopeRuntime is required for dropping container/pod metrics
	if isRuntimeInputEnabled(pipeline.Spec.Input) {
		processors = append(processors, "transform/set-instrumentation-scope-runtime")
	}

	processors = append(processors, makeInputSourceFiltersIDs(input)...)
	processors = append(processors, makeNamespaceFiltersIDs(input, pipeline)...)
	processors = append(processors, makeRuntimeResourcesFiltersIDs(input)...)
	processors = append(processors, makeDiagnosticMetricFiltersIDs(input)...)

	processors = append(processors, "transform/set-instrumentation-scope-kyma")

	processors = append(processors, "resource/insert-cluster-name", "resource/delete-skip-enrichment-attribute", "batch")

	return config.Pipeline{
		Receivers:  []string{formatRoutingConnectorID(pipeline.Name), formatForwardConnectorID(pipeline.Name)},
		Processors: processors,
		Exporters:  []string{formatOTLPExporterID(pipeline)},
	}
}

func makeReceiversIDs(pipeline *telemetryv1alpha1.MetricPipeline) []string {
	var receivers []string

	receivers = append(receivers, "otlp")

	receivers = append(receivers, "singleton_receiver_creator/kymastats")

	if isRuntimeInputEnabled(pipeline.Spec.Input) {
		receivers = append(receivers, "singleton_receiver_creator/k8s_cluster")
	}

	return receivers
}

func makeInputSourceFiltersIDs(input telemetryv1alpha1.MetricPipelineInput) []string {
	var processors []string

	if !isRuntimeInputEnabled(input) {
		processors = append(processors, "filter/drop-if-input-source-runtime")
	}

	if !isPrometheusInputEnabled(input) {
		processors = append(processors, "filter/drop-if-input-source-prometheus")
	}

	if !isIstioInputEnabled(input) {
		processors = append(processors, "filter/drop-if-input-source-istio")
	}

	if !isOtlpInputEnabled(input) {
		processors = append(processors, "filter/drop-if-input-source-otlp")
	}

	return processors
}

func makeNamespaceFiltersIDs(input telemetryv1alpha1.MetricPipelineInput, pipeline *telemetryv1alpha1.MetricPipeline) []string {
	var processors []string

	if isRuntimeInputEnabled(input) && shouldFilterByNamespace(input.Runtime.Namespaces) {
		processors = append(processors, formatNamespaceFilterID(pipeline.Name, metric.InputSourceRuntime))
	}

	if isPrometheusInputEnabled(input) && shouldFilterByNamespace(input.Prometheus.Namespaces) {
		processors = append(processors, formatNamespaceFilterID(pipeline.Name, metric.InputSourcePrometheus))
	}

	if isIstioInputEnabled(input) && shouldFilterByNamespace(input.Istio.Namespaces) {
		processors = append(processors, formatNamespaceFilterID(pipeline.Name, metric.InputSourceIstio))
	}

	if isOtlpInputEnabled(input) && input.Otlp != nil && shouldFilterByNamespace(input.Otlp.Namespaces) {
		processors = append(processors, formatNamespaceFilterID(pipeline.Name, metric.InputSourceOtlp))
	}

	return processors
}

func makeRuntimeResourcesFiltersIDs(input telemetryv1alpha1.MetricPipelineInput) []string {
	var processors []string

	if isRuntimeInputEnabled(input) && !isRuntimePodMetricsEnabled(input) {
		processors = append(processors, "filter/drop-runtime-pod-metrics")
	}

	if isRuntimeInputEnabled(input) && !isRuntimeContainerMetricsEnabled(input) {
		processors = append(processors, "filter/drop-runtime-container-metrics")
	}

	if isRuntimeInputEnabled(input) && !isRuntimeNodeMetricsEnabled(input) {
		processors = append(processors, "filter/drop-runtime-node-metrics")
	}

	if isRuntimeInputEnabled(input) {
		processors = append(processors, "filter/drop-k8s-cluster-metrics")
	}

	return processors
}

func makeDiagnosticMetricFiltersIDs(input telemetryv1alpha1.MetricPipelineInput) []string {
	var processors []string

	if isIstioInputEnabled(input) && !isIstioDiagnosticMetricsEnabled(input) {
		processors = append(processors, "filter/drop-diagnostic-metrics-if-input-source-istio")
	}

	if isPrometheusInputEnabled(input) && !isPrometheusDiagnosticMetricsEnabled(input) {
		processors = append(processors, "filter/drop-diagnostic-metrics-if-input-source-prometheus")
	}

	return processors
}

func formatOTLPExporterID(pipeline *telemetryv1alpha1.MetricPipeline) string {
	return otlpexporter.ExporterID(pipeline.Spec.Output.Otlp.Protocol, pipeline.Name)
}
