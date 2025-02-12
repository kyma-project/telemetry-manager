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
	metricpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/metricpipeline"
)

const (
	maxQueueSize = 256 // Maximum number of batches kept in memory before dropping
)

type Builder struct {
	Reader client.Reader
}

type BuildOptions struct {
	GatewayNamespace            string
	InstrumentationScopeVersion string
	ClusterName                 string
	CloudProvider               string
}

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.MetricPipeline, opts BuildOptions) (*Config, otlpexporter.EnvVars, error) {
	cfg := &Config{
		Base: config.Base{
			Service:    config.DefaultService(make(config.Pipelines)),
			Extensions: config.DefaultExtensions(),
		},
		Receivers:  makeReceiversConfig(),
		Processors: makeProcessorsConfig(opts),
		Exporters:  make(Exporters),
		Connectors: make(Connectors),
	}

	envVars := make(otlpexporter.EnvVars)

	queueSize := maxQueueSize / len(pipelines)

	for i := range pipelines {
		pipeline := pipelines[i]
		if pipeline.DeletionTimestamp != nil {
			continue
		}

		otlpExporterBuilder := otlpexporter.NewConfigBuilder(
			b.Reader,
			pipeline.Spec.Output.OTLP,
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

	if metricpipelineutils.IsPrometheusInputEnabled(input) && !metricpipelineutils.IsPrometheusDiagnosticInputEnabled(input) {
		cfg.Processors.DropDiagnosticMetricsIfInputSourcePrometheus = makeDropDiagnosticMetricsForInput(inputSourceEquals(metric.InputSourcePrometheus))
	}

	if metricpipelineutils.IsIstioInputEnabled(input) && !metricpipelineutils.IsIstioDiagnosticInputEnabled(input) {
		cfg.Processors.DropDiagnosticMetricsIfInputSourceIstio = makeDropDiagnosticMetricsForInput(inputSourceEquals(metric.InputSourceIstio))
	}
}

func declareInputSourceFilters(pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config) {
	input := pipeline.Spec.Input

	if !metricpipelineutils.IsRuntimeInputEnabled(input) {
		cfg.Processors.DropIfInputSourceRuntime = makeDropIfInputSourceRuntimeConfig()
	}

	if !metricpipelineutils.IsPrometheusInputEnabled(input) {
		cfg.Processors.DropIfInputSourcePrometheus = makeDropIfInputSourcePrometheusConfig()
	}

	if !metricpipelineutils.IsIstioInputEnabled(input) {
		cfg.Processors.DropIfInputSourceIstio = makeDropIfInputSourceIstioConfig()
	}

	if !metricpipelineutils.IsOTLPInputEnabled(input) {
		cfg.Processors.DropIfInputSourceOTLP = makeDropIfInputSourceOTLPConfig()
	}
}

func declareRuntimeResourcesFilters(pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config) {
	input := pipeline.Spec.Input

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimePodInputEnabled(input) {
		cfg.Processors.DropRuntimePodMetrics = makeDropRuntimePodMetricsConfig()
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeContainerInputEnabled(input) {
		cfg.Processors.DropRuntimeContainerMetrics = makeDropRuntimeContainerMetricsConfig()
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeNodeInputEnabled(input) {
		cfg.Processors.DropRuntimeNodeMetrics = makeDropRuntimeNodeMetricsConfig()
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeVolumeInputEnabled(input) {
		cfg.Processors.DropRuntimeVolumeMetrics = makeDropRuntimeVolumeMetricsConfig()
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeDeploymentInputEnabled(input) {
		cfg.Processors.DropRuntimeDeploymentMetrics = makeDropRuntimeDeploymentMetricsConfig()
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeStatefulSetInputEnabled(input) {
		cfg.Processors.DropRuntimeStatefulSetMetrics = makeDropRuntimeStatefulSetMetricsConfig()
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeDaemonSetInputEnabled(input) {
		cfg.Processors.DropRuntimeDaemonSetMetrics = makeDropRuntimeDaemonSetMetricsConfig()
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeJobInputEnabled(input) {
		cfg.Processors.DropRuntimeJobMetrics = makeDropRuntimeJobMetricsConfig()
	}
}

func declareNamespaceFilters(pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config) {
	if cfg.Processors.NamespaceFilters == nil {
		cfg.Processors.NamespaceFilters = make(NamespaceFilters)
	}

	input := pipeline.Spec.Input
	if metricpipelineutils.IsRuntimeInputEnabled(input) && shouldFilterByNamespace(input.Runtime.Namespaces) {
		processorID := formatNamespaceFilterID(pipeline.Name, metric.InputSourceRuntime)
		cfg.Processors.NamespaceFilters[processorID] = makeFilterByNamespaceConfig(pipeline.Spec.Input.Runtime.Namespaces, inputSourceEquals(metric.InputSourceRuntime))
	}

	if metricpipelineutils.IsPrometheusInputEnabled(input) && shouldFilterByNamespace(input.Prometheus.Namespaces) {
		processorID := formatNamespaceFilterID(pipeline.Name, metric.InputSourcePrometheus)
		cfg.Processors.NamespaceFilters[processorID] = makeFilterByNamespaceConfig(pipeline.Spec.Input.Prometheus.Namespaces, inputSourceEquals(metric.InputSourcePrometheus))
	}

	if metricpipelineutils.IsIstioInputEnabled(input) && shouldFilterByNamespace(input.Istio.Namespaces) {
		processorID := formatNamespaceFilterID(pipeline.Name, metric.InputSourceIstio)
		cfg.Processors.NamespaceFilters[processorID] = makeFilterByNamespaceConfig(pipeline.Spec.Input.Istio.Namespaces, inputSourceEquals(metric.InputSourceIstio))
	}

	if metricpipelineutils.IsOTLPInputEnabled(input) && input.OTLP != nil && shouldFilterByNamespace(input.OTLP.Namespaces) {
		processorID := formatNamespaceFilterID(pipeline.Name, metric.InputSourceOTLP)
		cfg.Processors.NamespaceFilters[processorID] = makeFilterByNamespaceConfig(pipeline.Spec.Input.OTLP.Namespaces, otlpInputSource())
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

	exporterID := otlpexporter.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
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
