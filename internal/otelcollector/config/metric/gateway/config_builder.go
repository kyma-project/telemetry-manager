package gateway

import (
	"context"
	"fmt"
	"maps"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/ottlexpr"
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
	ClusterUID                  string
	CloudProvider               string
	Enrichments                 *operatorv1alpha1.EnrichmentSpec
}

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.MetricPipeline, opts BuildOptions) (*Config, otlpexporter.EnvVars, error) {
	// Fill out the static parts of the config (global receivers, processors, exporters, connectors)
	cfg := &Config{
		Base: *config.DefaultBaseConfig(
			make(config.Pipelines),
			config.WithK8sLeaderElector("serviceAccount", "telemetry-metric-gateway-kymastats", opts.GatewayNamespace)),
		Receivers:  receiversConfig(),
		Processors: processorsConfig(opts),
		Exporters:  make(Exporters),
		Connectors: make(Connectors),
	}

	envVars := make(otlpexporter.EnvVars)

	queueSize := maxQueueSize / len(pipelines)

	// Iterate over each MetricPipeline CR and enrich the config with pipeline-specific components
	for i := range pipelines {
		pipeline := pipelines[i]

		otlpExporterBuilder := otlpexporter.NewConfigBuilder(
			b.Reader,
			pipeline.Spec.Output.OTLP,
			pipeline.Name,
			queueSize,
			otlpexporter.SignalTypeMetric,
		)
		if err := declareComponentsForMetricPipeline(ctx, otlpExporterBuilder, &pipeline, cfg, envVars); err != nil {
			return nil, nil, err
		}

		// Assemble the service pipelines for this MetricPipeline
		inputPipelineID := formatInputPipelineID(pipeline.Name)
		enrichmentPipelineID := formatAttributesEnrichmentPipelineID(pipeline.Name)
		outputPipelineID := formatOutputPipelineID(pipeline.Name)
		cfg.Service.Pipelines[inputPipelineID] = inputPipelineConfig(&pipeline)
		cfg.Service.Pipelines[enrichmentPipelineID] = enrichmentPipelineConfig(pipeline.Name)
		cfg.Service.Pipelines[outputPipelineID] = outputPipelineConfig(&pipeline)
	}

	// Return the assembled config and any environment variables needed for exporters
	return cfg, envVars, nil
}

// declareComponentsForMetricPipeline enriches a Config (receivers, processors, exporters etc.) with components for a given telemetryv1alpha1.MetricPipeline.
func declareComponentsForMetricPipeline(
	ctx context.Context,
	otlpExporterBuilder *otlpexporter.ConfigBuilder,
	pipeline *telemetryv1alpha1.MetricPipeline,
	cfg *Config,
	envVars otlpexporter.EnvVars,
) error {
	declareDiagnosticMetricsDropFilters(pipeline, cfg)
	declareInputSourceFilters(pipeline, cfg)
	declareRuntimeResourcesFilters(pipeline, cfg)
	declareNamespaceFilters(pipeline, cfg)
	declareConnectors(pipeline.Name, cfg)

	return declareOTLPExporter(ctx, otlpExporterBuilder, pipeline, cfg, envVars)
}

func declareDiagnosticMetricsDropFilters(pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config) {
	input := pipeline.Spec.Input

	if metricpipelineutils.IsPrometheusInputEnabled(input) && !metricpipelineutils.IsPrometheusDiagnosticInputEnabled(input) {
		cfg.Processors.DropDiagnosticMetricsIfInputSourcePrometheus = dropDiagnosticMetricsFilterConfig(inputSourceEquals(metric.InputSourcePrometheus))
	}

	if metricpipelineutils.IsIstioInputEnabled(input) && !metricpipelineutils.IsIstioDiagnosticInputEnabled(input) {
		cfg.Processors.DropDiagnosticMetricsIfInputSourceIstio = dropDiagnosticMetricsFilterConfig(inputSourceEquals(metric.InputSourceIstio))
	}
}

func declareInputSourceFilters(pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config) {
	input := pipeline.Spec.Input

	if !metricpipelineutils.IsRuntimeInputEnabled(input) {
		cfg.Processors.DropIfInputSourceRuntime = dropIfInputSourceRuntimeProcessorConfig()
	}

	if !metricpipelineutils.IsPrometheusInputEnabled(input) {
		cfg.Processors.DropIfInputSourcePrometheus = dropIfInputSourcePrometheusProcessorConfig()
	}

	if !metricpipelineutils.IsIstioInputEnabled(input) {
		cfg.Processors.DropIfInputSourceIstio = dropIfInputSourceIstioProcessorConfig()
	}

	if !metricpipelineutils.IsOTLPInputEnabled(input) {
		cfg.Processors.DropIfInputSourceOTLP = dropIfInputSourceOTLPProcessorConfig()
	}

	if !metricpipelineutils.IsIstioInputEnabled(input) || !metricpipelineutils.IsEnvoyMetricsEnabled(input) {
		cfg.Processors.DropIfEnvoyMetricsDisabled = dropIfEnvoyMetricsDisabledProcessorConfig()
	}
}

func declareRuntimeResourcesFilters(pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config) {
	input := pipeline.Spec.Input

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimePodInputEnabled(input) {
		cfg.Processors.DropRuntimePodMetrics = dropRuntimePodMetricsProcessorConfig()
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeContainerInputEnabled(input) {
		cfg.Processors.DropRuntimeContainerMetrics = dropRuntimeContainerMetricsProcessorConfig()
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeNodeInputEnabled(input) {
		cfg.Processors.DropRuntimeNodeMetrics = dropRuntimeNodeMetricsProcessorConfig()
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeVolumeInputEnabled(input) {
		cfg.Processors.DropRuntimeVolumeMetrics = dropRuntimeVolumeMetricsProcessorConfig()
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeDeploymentInputEnabled(input) {
		cfg.Processors.DropRuntimeDeploymentMetrics = dropRuntimeDeploymentMetricsProcessorConfig()
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeStatefulSetInputEnabled(input) {
		cfg.Processors.DropRuntimeStatefulSetMetrics = dropRuntimeStatefulSetMetricsProcessorConfig()
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeDaemonSetInputEnabled(input) {
		cfg.Processors.DropRuntimeDaemonSetMetrics = dropRuntimeDaemonSetMetricsProcessorConfig()
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeJobInputEnabled(input) {
		cfg.Processors.DropRuntimeJobMetrics = dropRuntimeJobMetricsProcessorConfig()
	}
}

func declareNamespaceFilters(pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config) {
	if cfg.Processors.NamespaceFilters == nil {
		cfg.Processors.NamespaceFilters = make(NamespaceFilters)
	}

	input := pipeline.Spec.Input
	if metricpipelineutils.IsRuntimeInputEnabled(input) && shouldFilterByNamespace(input.Runtime.Namespaces) {
		processorID := formatNamespaceFilterID(pipeline.Name, metric.InputSourceRuntime)
		cfg.Processors.NamespaceFilters[processorID] = filterByNamespaceProcessorConfig(pipeline.Spec.Input.Runtime.Namespaces, inputSourceEquals(metric.InputSourceRuntime))
	}

	if metricpipelineutils.IsPrometheusInputEnabled(input) && shouldFilterByNamespace(input.Prometheus.Namespaces) {
		processorID := formatNamespaceFilterID(pipeline.Name, metric.InputSourcePrometheus)
		cfg.Processors.NamespaceFilters[processorID] = filterByNamespaceProcessorConfig(pipeline.Spec.Input.Prometheus.Namespaces, ottlexpr.ResourceAttributeEquals(metric.KymaInputNameAttribute, metric.KymaInputPrometheus))
	}

	if metricpipelineutils.IsIstioInputEnabled(input) && shouldFilterByNamespace(input.Istio.Namespaces) {
		processorID := formatNamespaceFilterID(pipeline.Name, metric.InputSourceIstio)
		cfg.Processors.NamespaceFilters[processorID] = filterByNamespaceProcessorConfig(pipeline.Spec.Input.Istio.Namespaces, inputSourceEquals(metric.InputSourceIstio))
	}

	if metricpipelineutils.IsOTLPInputEnabled(input) && input.OTLP != nil && shouldFilterByNamespace(input.OTLP.Namespaces) {
		processorID := formatNamespaceFilterID(pipeline.Name, metric.InputSourceOTLP)
		cfg.Processors.NamespaceFilters[processorID] = filterByNamespaceProcessorConfig(pipeline.Spec.Input.OTLP.Namespaces, otlpInputSource())
	}
}

func declareConnectors(pipelineName string, cfg *Config) {
	forwardConnectorID := formatForwardConnectorID(pipelineName)
	cfg.Connectors[forwardConnectorID] = struct{}{}

	routingConnectorID := formatRoutingConnectorID(pipelineName)
	cfg.Connectors[routingConnectorID] = routingConnectorConfig(pipelineName)
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
