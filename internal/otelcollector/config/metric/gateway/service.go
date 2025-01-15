package gateway

import (
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
	metricpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/metricpipeline"
)

func makeInputPipelineServiceConfig(pipeline *telemetryv1alpha1.MetricPipeline) config.Pipeline {
	return config.Pipeline{
		Receivers:  makeReceiversIDs(),
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

func makeReceiversIDs() []string {
	var receivers []string

	receivers = append(receivers, "otlp")

	receivers = append(receivers, "singleton_receiver_creator/kymastats")

	return receivers
}

func makeInputSourceFiltersIDs(input telemetryv1alpha1.MetricPipelineInput) []string {
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

	if !metricpipelineutils.IsOTLPInputEnabled(input) {
		processors = append(processors, "filter/drop-if-input-source-otlp")
	}

	return processors
}

func makeNamespaceFiltersIDs(input telemetryv1alpha1.MetricPipelineInput, pipeline *telemetryv1alpha1.MetricPipeline) []string {
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

func makeRuntimeResourcesFiltersIDs(input telemetryv1alpha1.MetricPipelineInput) []string {
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

func makeDiagnosticMetricFiltersIDs(input telemetryv1alpha1.MetricPipelineInput) []string {
	var processors []string

	if metricpipelineutils.IsIstioInputEnabled(input) && !metricpipelineutils.IsIstioDiagnosticInputEnabled(input) {
		processors = append(processors, "filter/drop-diagnostic-metrics-if-input-source-istio")
	}

	if metricpipelineutils.IsPrometheusInputEnabled(input) && !metricpipelineutils.IsPrometheusDiagnosticInputEnabled(input) {
		processors = append(processors, "filter/drop-diagnostic-metrics-if-input-source-prometheus")
	}

	return processors
}

func formatOTLPExporterID(pipeline *telemetryv1alpha1.MetricPipeline) string {
	return otlpexporter.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
}
