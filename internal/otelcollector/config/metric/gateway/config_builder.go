package gateway

import (
	"context"
	"fmt"
	"maps"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
)

type Builder struct {
	Reader client.Reader
}

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.MetricPipeline, gatewayNamespace string, kymaInputAllowed bool) (*Config, otlpexporter.EnvVars, error) {
	cfg := &Config{
		Base: config.Base{
			Service:    config.DefaultService(make(config.Pipelines)),
			Extensions: config.DefaultExtensions(),
		},
		Receivers:  makeReceiversConfig(),
		Processors: makeProcessorsConfig(),
		Exporters:  make(Exporters),
	}

	envVars := make(otlpexporter.EnvVars)
	queueSize := 256 / len(pipelines)

	for i := range pipelines {
		pipeline := pipelines[i]
		if pipeline.DeletionTimestamp != nil {
			continue
		}

		otlpExporterBuilder := otlpexporter.NewConfigBuilder(
			b.Reader,
			pipeline.Spec.Output.Otlp,
			pipeline.Name,
			queueSize,
			otlpexporter.SignalTypeMetric,
		)
		if err := declareComponentsForMetricPipeline(ctx, otlpExporterBuilder, &pipeline, cfg, envVars, gatewayNamespace, kymaInputAllowed); err != nil {
			return nil, nil, err
		}

		pipelineID := fmt.Sprintf("metrics/%s", pipeline.Name)
		cfg.Service.Pipelines[pipelineID] = makeServicePipelineConfig(&pipeline, kymaInputAllowed)
	}

	return cfg, envVars, nil
}

// declareComponentsForMetricPipeline enriches a Config (receivers, processors, exporters etc.) with components for a given telemetryv1alpha1.MetricPipeline.
func declareComponentsForMetricPipeline(
	ctx context.Context,
	otlpExporterBuilder *otlpexporter.ConfigBuilder,
	pipeline *telemetryv1alpha1.MetricPipeline,
	cfg *Config,
	envVars otlpexporter.EnvVars,
	gatewayNamespace string,
	kymaInputAllowed bool,
) error {
	declareSingletonKymaStatsReceiverCreator(pipeline, cfg, gatewayNamespace, kymaInputAllowed)
	declareDiagnosticMetricsDropFilters(pipeline, cfg)
	declareInputSourceFilters(pipeline, cfg)
	declareRuntimeResourcesFilters(pipeline, cfg)
	declareNamespaceFilters(pipeline, cfg)
	return declareOTLPExporter(ctx, otlpExporterBuilder, pipeline, cfg, envVars)
}

func declareSingletonKymaStatsReceiverCreator(pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config, gatewayNamespace string, kymaInputAllowed bool) {
	if isKymaInputEnabled(pipeline.Annotations, kymaInputAllowed) {
		cfg.Receivers.SingletonKymaStatsReceiverCreator = makeSingletonKymaStatsReceiverCreatorConfig(gatewayNamespace)
	}
}

func declareDiagnosticMetricsDropFilters(pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config) {
	input := pipeline.Spec.Input

	if isPrometheusInputEnabled(input) && !isPrometheusDiagnosticMetricsEnabled(input) {
		cfg.Processors.DropDiagnosticMetricsIfInputSourcePrometheus = makeDropDiagnosticMetricsForInput(inputSourceEquals(metric.InputSourcePrometheus))
	}
	if isIstioInputEnabled(input) && !isIstioDiagnosticMetricsEnabled(input) {
		cfg.Processors.DropDiagnosticMetricsIfInputSourceIstio = makeDropDiagnosticMetricsForInput(inputSourceEquals(metric.InputSourceIstio))
	}
}

func declareInputSourceFilters(pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config) {
	input := pipeline.Spec.Input

	if !isRuntimeInputEnabled(input) {
		cfg.Processors.DropIfInputSourceRuntime = makeDropIfInputSourceRuntimeConfig()
	}
	if !isPrometheusInputEnabled(input) {
		cfg.Processors.DropIfInputSourcePrometheus = makeDropIfInputSourcePrometheusConfig()
	}
	if !isIstioInputEnabled(input) {
		cfg.Processors.DropIfInputSourceIstio = makeDropIfInputSourceIstioConfig()
	}
	if !isOtlpInputEnabled(input) {
		cfg.Processors.DropIfInputSourceOtlp = makeDropIfInputSourceOtlpConfig()
	}
}

func declareRuntimeResourcesFilters(pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config) {
	input := pipeline.Spec.Input

	if isRuntimeInputEnabled(input) && !isRuntimePodMetricsEnabled(input) {
		cfg.Processors.DropRuntimePodMetrics = makeDropRuntimePodMetricsConfig()
	}
	if isRuntimeInputEnabled(input) && !isRuntimeContainerMetricsEnabled(input) {
		cfg.Processors.DropRuntimeContainerMetrics = makeDropRuntimeContainerMetricsConfig()
	}
}

func declareNamespaceFilters(pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config) {
	if cfg.Processors.NamespaceFilters == nil {
		cfg.Processors.NamespaceFilters = make(NamespaceFilters)
	}

	input := pipeline.Spec.Input
	if isRuntimeInputEnabled(input) && shouldFilterByNamespace(input.Runtime.Namespaces) {
		processorID := formatNamespaceFilterID(pipeline.Name, metric.InputSourceRuntime)
		cfg.Processors.NamespaceFilters[processorID] = makeFilterByNamespaceRuntimeInputConfig(pipeline.Spec.Input.Runtime.Namespaces)
	}
	if isPrometheusInputEnabled(input) && shouldFilterByNamespace(input.Prometheus.Namespaces) {
		processorID := formatNamespaceFilterID(pipeline.Name, metric.InputSourcePrometheus)
		cfg.Processors.NamespaceFilters[processorID] = makeFilterByNamespacePrometheusInputConfig(pipeline.Spec.Input.Prometheus.Namespaces)
	}
	if isIstioInputEnabled(input) && shouldFilterByNamespace(input.Istio.Namespaces) {
		processorID := formatNamespaceFilterID(pipeline.Name, metric.InputSourceIstio)
		cfg.Processors.NamespaceFilters[processorID] = makeFilterByNamespaceIstioInputConfig(pipeline.Spec.Input.Istio.Namespaces)
	}
	if isOtlpInputEnabled(input) && input.Otlp != nil && shouldFilterByNamespace(input.Otlp.Namespaces) {
		processorID := formatNamespaceFilterID(pipeline.Name, metric.InputSourceOtlp)
		cfg.Processors.NamespaceFilters[processorID] = makeFilterByNamespaceOtlpInputConfig(pipeline.Spec.Input.Otlp.Namespaces)
	}
}

func declareOTLPExporter(ctx context.Context, otlpExporterBuilder *otlpexporter.ConfigBuilder, pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config, envVars otlpexporter.EnvVars) error {
	otlpExporterConfig, otlpExporterEnvVars, err := otlpExporterBuilder.MakeConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to make otlp exporter config: %w", err)
	}

	maps.Copy(envVars, otlpExporterEnvVars)

	exporterID := otlpexporter.ExporterID(pipeline.Spec.Output.Otlp.Protocol, pipeline.Name)
	cfg.Exporters[exporterID] = Exporter{OTLP: otlpExporterConfig}

	return nil
}

func makeServicePipelineConfig(pipeline *telemetryv1alpha1.MetricPipeline, kymaInputAllowed bool) config.Pipeline {
	processors := []string{"memory_limiter", "k8sattributes"}

	input := pipeline.Spec.Input

	processors = append(processors, makeInputSourceFiltersIDs(input)...)
	processors = append(processors, makeNamespaceFiltersIDs(input, pipeline)...)
	processors = append(processors, makeRuntimeResourcesFiltersIDs(input)...)
	processors = append(processors, makeDiagnosticMetricFiltersIDs(input)...)

	processors = append(processors, "resource/insert-cluster-name", "transform/resolve-service-name", "batch")

	return config.Pipeline{
		Receivers:  makeReceiversIDs(pipeline.Annotations, kymaInputAllowed),
		Processors: processors,
		Exporters:  []string{makeOTLPExporterID(pipeline)},
	}
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

func shouldFilterByNamespace(namespaceSelector *telemetryv1alpha1.MetricPipelineInputNamespaceSelector) bool {
	return namespaceSelector != nil && (len(namespaceSelector.Include) > 0 || len(namespaceSelector.Exclude) > 0)
}

func formatNamespaceFilterID(pipelineName string, inputSourceType metric.InputSourceType) string {
	return fmt.Sprintf("filter/%s-filter-by-namespace-%s-input", pipelineName, inputSourceType)
}

func makeReceiversIDs(annotations map[string]string, kymaInputAllowed bool) []string {
	var receivers []string

	receivers = append(receivers, "otlp")

	if isKymaInputEnabled(annotations, kymaInputAllowed) {
		receivers = append(receivers, "singleton_receiver_creator/kymastats")
	}

	return receivers
}

func makeOTLPExporterID(pipeline *telemetryv1alpha1.MetricPipeline) string {
	return otlpexporter.ExporterID(pipeline.Spec.Output.Otlp.Protocol, pipeline.Name)
}

func isPrometheusInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Prometheus != nil && input.Prometheus.Enabled
}

func isRuntimeInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Runtime != nil && input.Runtime.Enabled
}

func isIstioInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Istio != nil && input.Istio.Enabled
}

func isOtlpInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Otlp == nil || !input.Otlp.Disabled
}

func isPrometheusDiagnosticMetricsEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Prometheus.DiagnosticMetrics != nil && input.Prometheus.DiagnosticMetrics.Enabled
}

func isIstioDiagnosticMetricsEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Istio.DiagnosticMetrics != nil && input.Istio.DiagnosticMetrics.Enabled
}

func isRuntimePodMetricsEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	// Define first isRuntimePodMetricsDisabled to ensure that the runtime pod metrics will be enabled by default
	// in case any of the fields (Resources, Pod or Enabled) is nil
	isRuntimePodMetricsDisabled := input.Runtime.Resources != nil &&
		input.Runtime.Resources.Pod != nil &&
		input.Runtime.Resources.Pod.Enabled != nil &&
		!*input.Runtime.Resources.Pod.Enabled

	return !isRuntimePodMetricsDisabled
}

func isRuntimeContainerMetricsEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	// Define first isRuntimeContainerMetricsDisabled to ensure that the runtime container metrics will be enabled by default
	// in case any of the fields (Resources, Pod or Enabled) is nil
	isRuntimeContainerMetricsDisabled := input.Runtime.Resources != nil &&
		input.Runtime.Resources.Container != nil &&
		input.Runtime.Resources.Container.Enabled != nil &&
		!*input.Runtime.Resources.Container.Enabled

	return !isRuntimeContainerMetricsDisabled
}

func isKymaInputEnabled(annotations map[string]string, kymaInputAllowed bool) bool {
	return kymaInputAllowed && annotations["experimental-kyma-input"] == "true"
}
