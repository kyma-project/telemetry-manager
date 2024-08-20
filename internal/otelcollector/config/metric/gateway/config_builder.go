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

const KymaInputAnnotation = "telemetry.kyma-project.io/experimental-kyma-input"

type Builder struct {
	Reader client.Reader
}

type BuildOptions struct {
	GatewayNamespace            string
	InstrumentationScopeVersion string
	KymaInputAllowed            bool
	K8sClusterReceiverAllowed   bool
}

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.MetricPipeline, opts BuildOptions) (*Config, otlpexporter.EnvVars, error) {
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
		if err := declareComponentsForMetricPipeline(ctx, otlpExporterBuilder, &pipeline, cfg, envVars, opts); err != nil {
			return nil, nil, err
		}

		pipelineID := fmt.Sprintf("metrics/%s", pipeline.Name)
		cfg.Service.Pipelines[pipelineID] = makeServicePipelineConfig(&pipeline, opts)
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
	opts BuildOptions,
) error {
	declareSingletonKymaStatsReceiverCreator(pipeline, cfg, opts)
	declareSingletonK8sClusterReceiverCreator(pipeline, cfg, opts)
	declareDiagnosticMetricsDropFilters(pipeline, cfg)
	declareInputSourceFilters(pipeline, cfg)
	declareRuntimeResourcesFilters(pipeline, cfg, opts)
	declareNamespaceFilters(pipeline, cfg)
	declareInstrumentationScopeTransform(pipeline, cfg, opts)
	return declareOTLPExporter(ctx, otlpExporterBuilder, pipeline, cfg, envVars)
}

func declareSingletonK8sClusterReceiverCreator(pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config, opts BuildOptions) {
	if isRuntimeInputEnabled(pipeline.Spec.Input) && opts.K8sClusterReceiverAllowed {
		cfg.Receivers.SingletonK8sClusterReceiverCreator = makeSingletonK8sClusterReceiverCreatorConfig(opts.GatewayNamespace)
	}
}

func declareSingletonKymaStatsReceiverCreator(pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config, opts BuildOptions) {
	if isKymaInputEnabled(pipeline.Annotations, opts.KymaInputAllowed) {
		cfg.Receivers.SingletonKymaStatsReceiverCreator = makeSingletonKymaStatsReceiverCreatorConfig(opts.GatewayNamespace)
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

func declareRuntimeResourcesFilters(pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config, opts BuildOptions) {
	input := pipeline.Spec.Input

	if isRuntimeInputEnabled(input) && !isRuntimePodMetricsEnabled(input) {
		cfg.Processors.DropRuntimePodMetrics = makeDropRuntimePodMetricsConfig()
	}
	if isRuntimeInputEnabled(input) && !isRuntimeContainerMetricsEnabled(input) {
		cfg.Processors.DropRuntimeContainerMetrics = makeDropRuntimeContainerMetricsConfig()
	}

	if isRuntimeInputEnabled(input) && opts.K8sClusterReceiverAllowed {
		cfg.Processors.DropK8sClusterMetrics = makeK8sClusterDropMetrics()
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

func declareInstrumentationScopeTransform(pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config, opts BuildOptions) {
	if isKymaInputEnabled(pipeline.Annotations, opts.KymaInputAllowed) {
		cfg.Processors.SetInstrumentationScopeKyma = metric.MakeInstrumentationScopeProcessor(metric.InputSourceKyma, opts.InstrumentationScopeVersion)
	}
	if isRuntimeInputEnabled(pipeline.Spec.Input) && opts.K8sClusterReceiverAllowed {
		cfg.Processors.SetInstrumentationScopeRuntime = metric.MakeInstrumentationScopeProcessor(metric.InputSourceK8sCluster, opts.InstrumentationScopeVersion)
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

func makeServicePipelineConfig(pipeline *telemetryv1alpha1.MetricPipeline, opts BuildOptions) config.Pipeline {
	processors := []string{"memory_limiter", "k8sattributes"}

	input := pipeline.Spec.Input

	// Perform the transform before runtime resource filter as InstrumentationScopeK8sCluster is required for dropping container/pod metrics
	if isRuntimeInputEnabled(pipeline.Spec.Input) && opts.K8sClusterReceiverAllowed {
		processors = append(processors, "transform/set-instrumentation-scope-k8s_cluster")
	}

	processors = append(processors, makeInputSourceFiltersIDs(input)...)
	processors = append(processors, makeNamespaceFiltersIDs(input, pipeline)...)
	processors = append(processors, makeRuntimeResourcesFiltersIDs(input, opts)...)
	processors = append(processors, makeDiagnosticMetricFiltersIDs(input)...)

	if isKymaInputEnabled(pipeline.Annotations, opts.KymaInputAllowed) {
		processors = append(processors, "transform/set-instrumentation-scope-kyma")
	}

	processors = append(processors, "resource/insert-cluster-name", "transform/resolve-service-name", "batch")

	return config.Pipeline{
		Receivers:  makeReceiversIDs(pipeline, opts),
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

func makeRuntimeResourcesFiltersIDs(input telemetryv1alpha1.MetricPipelineInput, opts BuildOptions) []string {
	var processors []string

	if isRuntimeInputEnabled(input) && !isRuntimePodMetricsEnabled(input) {
		processors = append(processors, "filter/drop-runtime-pod-metrics")
	}
	if isRuntimeInputEnabled(input) && !isRuntimeContainerMetricsEnabled(input) {
		processors = append(processors, "filter/drop-runtime-container-metrics")
	}
	if isRuntimeInputEnabled(input) && opts.K8sClusterReceiverAllowed {
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

func shouldFilterByNamespace(namespaceSelector *telemetryv1alpha1.MetricPipelineInputNamespaceSelector) bool {
	return namespaceSelector != nil && (len(namespaceSelector.Include) > 0 || len(namespaceSelector.Exclude) > 0)
}

func formatNamespaceFilterID(pipelineName string, inputSourceType metric.InputSourceType) string {
	return fmt.Sprintf("filter/%s-filter-by-namespace-%s-input", pipelineName, inputSourceType)
}

func makeReceiversIDs(pipeline *telemetryv1alpha1.MetricPipeline, opts BuildOptions) []string {
	var receivers []string

	receivers = append(receivers, "otlp")

	if isKymaInputEnabled(pipeline.Annotations, opts.KymaInputAllowed) {
		receivers = append(receivers, "singleton_receiver_creator/kymastats")
	}

	if isRuntimeInputEnabled(pipeline.Spec.Input) && opts.K8sClusterReceiverAllowed {
		receivers = append(receivers, "singleton_receiver_creator/k8s_cluster")
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
	return kymaInputAllowed && annotations[KymaInputAnnotation] == "true"
}

//func isK8sClusterReceiverEnabled(input telemetryv1alpha1.MetricPipelineInput, k8sClusterReceiverAllowed bool) bool {
//	return k8sClusterReceiverAllowed && input.Runtime != nil && input.Runtime.Enabled
//}
