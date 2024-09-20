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

type BuildOptions struct {
	GatewayNamespace            string
	InstrumentationScopeVersion string
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
		Connectors: make(Connectors),
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

		inputPipelineID := formatInputPipelineID(pipeline.Name)
		attributesEnrichmentPipelineID := formatAttributesEnrichmentPipelineID(pipeline.Name)
		outputPipelineID := formatOutputPipelineID(pipeline.Name)
		cfg.Service.Pipelines[inputPipelineID] = makeInputPipelineServiceConfig(&pipeline, opts)
		cfg.Service.Pipelines[attributesEnrichmentPipelineID] = makeAttributesEnrichmentPipelineServiceConfig(pipeline.Name)
		cfg.Service.Pipelines[outputPipelineID] = makeOutputPipelineServiceConfig(&pipeline, opts)
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
	declareSingletonKymaStatsReceiverCreator(cfg, opts)
	declareSingletonK8sClusterReceiverCreator(pipeline, cfg, opts)
	declareDiagnosticMetricsDropFilters(pipeline, cfg)
	declareInputSourceFilters(pipeline, cfg)
	declareRuntimeResourcesFilters(pipeline, cfg)
	declareNamespaceFilters(pipeline, cfg)
	declareInstrumentationScopeTransform(pipeline, cfg, opts)
	declareConnectors(pipeline.Name, cfg)
	return declareOTLPExporter(ctx, otlpExporterBuilder, pipeline, cfg, envVars)
}

func declareSingletonK8sClusterReceiverCreator(pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config, opts BuildOptions) {
	if isRuntimeInputEnabled(pipeline.Spec.Input) {
		cfg.Receivers.SingletonK8sClusterReceiverCreator = makeSingletonK8sClusterReceiverCreatorConfig(opts.GatewayNamespace)
	}
}

func declareSingletonKymaStatsReceiverCreator(cfg *Config, opts BuildOptions) {
	cfg.Receivers.SingletonKymaStatsReceiverCreator = makeSingletonKymaStatsReceiverCreatorConfig(opts.GatewayNamespace)

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

	if isRuntimeInputEnabled(input) {
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
	cfg.Processors.SetInstrumentationScopeKyma = metric.MakeInstrumentationScopeProcessor(metric.InputSourceKyma, opts.InstrumentationScopeVersion)

	if isRuntimeInputEnabled(pipeline.Spec.Input) {
		cfg.Processors.SetInstrumentationScopeRuntime = metric.MakeInstrumentationScopeProcessor(metric.InputSourceK8sCluster, opts.InstrumentationScopeVersion)
	}
}

func declareConnectors(pipelineName string, cfg *Config) {
	forwardConnectorID := formatForwardConnectorID(pipelineName)
	cfg.Connectors[forwardConnectorID] = struct{}{}

	routingConnectorID := formatRoutingConnectorID(pipelineName)
	cfg.Connectors[routingConnectorID] = makeRoutingConnectorConfig(pipelineName)
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

func shouldFilterByNamespace(namespaceSelector *telemetryv1alpha1.MetricPipelineInputNamespaceSelector) bool {
	return namespaceSelector != nil && (len(namespaceSelector.Include) > 0 || len(namespaceSelector.Exclude) > 0)
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

func formatInputPipelineID(pipelineName string) string {
	return fmt.Sprintf("metrics/%s-input", pipelineName)
}

func formatAttributesEnrichmentPipelineID(pipelineName string) string {
	return fmt.Sprintf("metrics/%s-attributes-enrichment", pipelineName)
}

func formatOutputPipelineID(pipelineName string) string {
	return fmt.Sprintf("metrics/%s-output", pipelineName)
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
