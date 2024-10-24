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

	const maxQueueSize = 256
	queueSize := maxQueueSize / len(pipelines)

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
		cfg.Service.Pipelines[inputPipelineID] = makeInputPipelineServiceConfig(&pipeline)
		cfg.Service.Pipelines[attributesEnrichmentPipelineID] = makeAttributesEnrichmentPipelineServiceConfig(pipeline.Name)
		cfg.Service.Pipelines[outputPipelineID] = makeOutputPipelineServiceConfig(&pipeline)
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
	declareDiagnosticMetricsDropFilters(pipeline, cfg)
	declareInputSourceFilters(pipeline, cfg)
	declareRuntimeResourcesFilters(pipeline, cfg)
	declareNamespaceFilters(pipeline, cfg)
	declareInstrumentationScopeTransform(cfg, opts)
	declareConnectors(pipeline.Name, cfg)

	return declareOTLPExporter(ctx, otlpExporterBuilder, pipeline, cfg, envVars)
}

func declareSingletonKymaStatsReceiverCreator(cfg *Config, opts BuildOptions) {
	cfg.Receivers.SingletonKymaStatsReceiverCreator = makeSingletonKymaStatsReceiverCreatorConfig(opts.GatewayNamespace)
}

func declareDiagnosticMetricsDropFilters(pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config) {
	input := pipeline.Spec.Input

	if metric.IsPrometheusInputEnabled(input) && !metric.IsPrometheusDiagnosticInputEnabled(input) {
		cfg.Processors.DropDiagnosticMetricsIfInputSourcePrometheus = makeDropDiagnosticMetricsForInput(inputSourceEquals(metric.InputSourcePrometheus))
	}

	if metric.IsIstioInputEnabled(input) && !metric.IsIstioDiagnosticInputEnabled(input) {
		cfg.Processors.DropDiagnosticMetricsIfInputSourceIstio = makeDropDiagnosticMetricsForInput(inputSourceEquals(metric.InputSourceIstio))
	}
}

func declareInputSourceFilters(pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config) {
	input := pipeline.Spec.Input

	if !metric.IsRuntimeInputEnabled(input) {
		cfg.Processors.DropIfInputSourceRuntime = makeDropIfInputSourceRuntimeConfig()
	}

	if !metric.IsPrometheusInputEnabled(input) {
		cfg.Processors.DropIfInputSourcePrometheus = makeDropIfInputSourcePrometheusConfig()
	}

	if !metric.IsIstioInputEnabled(input) {
		cfg.Processors.DropIfInputSourceIstio = makeDropIfInputSourceIstioConfig()
	}

	if !metric.IsOTLPInputEnabled(input) {
		cfg.Processors.DropIfInputSourceOtlp = makeDropIfInputSourceOtlpConfig()
	}
}

func declareRuntimeResourcesFilters(pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config) {
	input := pipeline.Spec.Input

	if metric.IsRuntimeInputEnabled(input) && !metric.IsRuntimePodInputEnabled(input) {
		cfg.Processors.DropRuntimePodMetrics = makeDropRuntimePodMetricsConfig()
	}

	if metric.IsRuntimeInputEnabled(input) && !metric.IsRuntimeContainerInputEnabled(input) {
		cfg.Processors.DropRuntimeContainerMetrics = makeDropRuntimeContainerMetricsConfig()
	}

	if metric.IsRuntimeInputEnabled(input) && !metric.IsRuntimeNodeInputEnabled(input) {
		cfg.Processors.DropRuntimeNodeMetrics = makeDropRuntimeNodeMetricsConfig()
	}

	if metric.IsRuntimeInputEnabled(input) && !metric.IsRuntimeVolumeInputEnabled(input) {
		cfg.Processors.DropRuntimeVolumeMetrics = makeDropRuntimeVolumeMetricsConfig()
	}

	if metric.IsRuntimeInputEnabled(input) && !metric.IsRuntimeDeploymentInputEnabled(input) {
		cfg.Processors.DropRuntimeDeploymentMetrics = makeDropRuntimeDeploymentMetricsConfig()
	}

	if metric.IsRuntimeInputEnabled(input) && !metric.IsRuntimeStatefulSetInputEnabled(input) {
		cfg.Processors.DropRuntimeStatefulSetMetrics = makeDropRuntimeStatefulSetMetricsConfig()
	}

	if metric.IsRuntimeInputEnabled(input) && !metric.IsRuntimeDaemonSetInputEnabled(input) {
		cfg.Processors.DropRuntimeDaemonSetMetrics = makeDropRuntimeDaemonSetMetricsConfig()
	}

	if metric.IsRuntimeInputEnabled(input) && !metric.IsRuntimeJobInputEnabled(input) {
		cfg.Processors.DropRuntimeJobMetrics = makeDropRuntimeJobMetricsConfig()
	}
}

func declareNamespaceFilters(pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config) {
	if cfg.Processors.NamespaceFilters == nil {
		cfg.Processors.NamespaceFilters = make(NamespaceFilters)
	}

	input := pipeline.Spec.Input
	if metric.IsRuntimeInputEnabled(input) && shouldFilterByNamespace(input.Runtime.Namespaces) {
		processorID := formatNamespaceFilterID(pipeline.Name, metric.InputSourceRuntime)
		cfg.Processors.NamespaceFilters[processorID] = makeFilterByNamespaceConfig(pipeline.Spec.Input.Runtime.Namespaces, inputSourceEquals(metric.InputSourceRuntime))
	}

	if metric.IsPrometheusInputEnabled(input) && shouldFilterByNamespace(input.Prometheus.Namespaces) {
		processorID := formatNamespaceFilterID(pipeline.Name, metric.InputSourcePrometheus)
		cfg.Processors.NamespaceFilters[processorID] = makeFilterByNamespaceConfig(pipeline.Spec.Input.Prometheus.Namespaces, inputSourceEquals(metric.InputSourcePrometheus))
	}

	if metric.IsIstioInputEnabled(input) && shouldFilterByNamespace(input.Istio.Namespaces) {
		processorID := formatNamespaceFilterID(pipeline.Name, metric.InputSourceIstio)
		cfg.Processors.NamespaceFilters[processorID] = makeFilterByNamespaceConfig(pipeline.Spec.Input.Istio.Namespaces, inputSourceEquals(metric.InputSourceIstio))
	}

	if metric.IsOTLPInputEnabled(input) && input.Otlp != nil && shouldFilterByNamespace(input.Otlp.Namespaces) {
		processorID := formatNamespaceFilterID(pipeline.Name, metric.InputSourceOtlp)
		cfg.Processors.NamespaceFilters[processorID] = makeFilterByNamespaceConfig(pipeline.Spec.Input.Otlp.Namespaces, otlpInputSource())
	}
}

func declareInstrumentationScopeTransform(cfg *Config, opts BuildOptions) {
	cfg.Processors.SetInstrumentationScopeKyma = metric.MakeInstrumentationScopeProcessor(opts.InstrumentationScopeVersion, metric.InputSourceKyma)
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

func shouldFilterByNamespace(namespaceSelector *telemetryv1alpha1.NamespaceSelector) bool {
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
