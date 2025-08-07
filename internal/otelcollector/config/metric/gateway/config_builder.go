package gateway

import (
	"context"
	"fmt"
	"maps"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
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

	config  *Config
	envVars otlpexporter.EnvVars
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
	b.config = newConfig(opts)
	b.envVars = make(otlpexporter.EnvVars)

	// Iterate over each MetricPipeline CR and enrich the config with pipeline-specific components
	queueSize := maxQueueSize / len(pipelines)

	for i := range pipelines {
		pipeline := pipelines[i]

		if err := b.addComponents(ctx, &pipeline, queueSize); err != nil {
			return nil, nil, err
		}

		// Add input, output, and enrichment pipelines to the service config
		b.addServicePipelines(&pipeline)
	}

	return b.config, b.envVars, nil
}

// addComponents enriches a Config (receivers, processors, exporters etc.) with components for a given telemetryv1alpha1.MetricPipeline.
func (b *Builder) addComponents(
	ctx context.Context,
	pipeline *telemetryv1alpha1.MetricPipeline,
	queueSize int,
) error {
	b.addDiagnosticMetricsDropFilters(pipeline)
	b.addInputSourceFilters(pipeline)
	b.addRuntimeResourcesFilters(pipeline)
	b.addNamespaceFilters(pipeline)
	b.addConnectors(pipeline.Name)

	return b.addOTLPExporter(ctx, pipeline, queueSize)
}

func (b *Builder) addDiagnosticMetricsDropFilters(pipeline *telemetryv1alpha1.MetricPipeline) {
	input := pipeline.Spec.Input

	if metricpipelineutils.IsPrometheusInputEnabled(input) && !metricpipelineutils.IsPrometheusDiagnosticInputEnabled(input) {
		b.config.Processors.DropDiagnosticMetricsIfInputSourcePrometheus = dropDiagnosticMetricsFilterConfig(inputSourceEquals(metric.InputSourcePrometheus))
	}

	if metricpipelineutils.IsIstioInputEnabled(input) && !metricpipelineutils.IsIstioDiagnosticInputEnabled(input) {
		b.config.Processors.DropDiagnosticMetricsIfInputSourceIstio = dropDiagnosticMetricsFilterConfig(inputSourceEquals(metric.InputSourceIstio))
	}
}

func (b *Builder) addInputSourceFilters(pipeline *telemetryv1alpha1.MetricPipeline) {
	input := pipeline.Spec.Input

	if !metricpipelineutils.IsRuntimeInputEnabled(input) {
		b.config.Processors.DropIfInputSourceRuntime = dropIfInputSourceRuntimeProcessorConfig()
	}

	if !metricpipelineutils.IsPrometheusInputEnabled(input) {
		b.config.Processors.DropIfInputSourcePrometheus = dropIfInputSourcePrometheusProcessorConfig()
	}

	if !metricpipelineutils.IsIstioInputEnabled(input) {
		b.config.Processors.DropIfInputSourceIstio = dropIfInputSourceIstioProcessorConfig()
	}

	if !metricpipelineutils.IsOTLPInputEnabled(input) {
		b.config.Processors.DropIfInputSourceOTLP = dropIfInputSourceOTLPProcessorConfig()
	}

	if !metricpipelineutils.IsIstioInputEnabled(input) || !metricpipelineutils.IsEnvoyMetricsEnabled(input) {
		b.config.Processors.DropIfEnvoyMetricsDisabled = dropIfEnvoyMetricsDisabledProcessorConfig()
	}
}

func (b *Builder) addRuntimeResourcesFilters(pipeline *telemetryv1alpha1.MetricPipeline) {
	input := pipeline.Spec.Input

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimePodInputEnabled(input) {
		b.config.Processors.DropRuntimePodMetrics = dropRuntimePodMetricsProcessorConfig()
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeContainerInputEnabled(input) {
		b.config.Processors.DropRuntimeContainerMetrics = dropRuntimeContainerMetricsProcessorConfig()
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeNodeInputEnabled(input) {
		b.config.Processors.DropRuntimeNodeMetrics = dropRuntimeNodeMetricsProcessorConfig()
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeVolumeInputEnabled(input) {
		b.config.Processors.DropRuntimeVolumeMetrics = dropRuntimeVolumeMetricsProcessorConfig()
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeDeploymentInputEnabled(input) {
		b.config.Processors.DropRuntimeDeploymentMetrics = dropRuntimeDeploymentMetricsProcessorConfig()
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeStatefulSetInputEnabled(input) {
		b.config.Processors.DropRuntimeStatefulSetMetrics = dropRuntimeStatefulSetMetricsProcessorConfig()
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeDaemonSetInputEnabled(input) {
		b.config.Processors.DropRuntimeDaemonSetMetrics = dropRuntimeDaemonSetMetricsProcessorConfig()
	}

	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeJobInputEnabled(input) {
		b.config.Processors.DropRuntimeJobMetrics = dropRuntimeJobMetricsProcessorConfig()
	}
}

func (b *Builder) addNamespaceFilters(pipeline *telemetryv1alpha1.MetricPipeline) {
	if b.config.Processors.NamespaceFilters == nil {
		b.config.Processors.NamespaceFilters = make(NamespaceFilters)
	}

	input := pipeline.Spec.Input
	if metricpipelineutils.IsRuntimeInputEnabled(input) && shouldFilterByNamespace(input.Runtime.Namespaces) {
		processorID := formatNamespaceFilterID(pipeline.Name, metric.InputSourceRuntime)
		b.config.Processors.NamespaceFilters[processorID] = filterByNamespaceProcessorConfig(pipeline.Spec.Input.Runtime.Namespaces, inputSourceEquals(metric.InputSourceRuntime))
	}

	if metricpipelineutils.IsPrometheusInputEnabled(input) && shouldFilterByNamespace(input.Prometheus.Namespaces) {
		processorID := formatNamespaceFilterID(pipeline.Name, metric.InputSourcePrometheus)
		b.config.Processors.NamespaceFilters[processorID] = filterByNamespaceProcessorConfig(pipeline.Spec.Input.Prometheus.Namespaces, ottlexpr.ResourceAttributeEquals(metric.KymaInputNameAttribute, metric.KymaInputPrometheus))
	}

	if metricpipelineutils.IsIstioInputEnabled(input) && shouldFilterByNamespace(input.Istio.Namespaces) {
		processorID := formatNamespaceFilterID(pipeline.Name, metric.InputSourceIstio)
		b.config.Processors.NamespaceFilters[processorID] = filterByNamespaceProcessorConfig(pipeline.Spec.Input.Istio.Namespaces, inputSourceEquals(metric.InputSourceIstio))
	}

	if metricpipelineutils.IsOTLPInputEnabled(input) && input.OTLP != nil && shouldFilterByNamespace(input.OTLP.Namespaces) {
		processorID := formatNamespaceFilterID(pipeline.Name, metric.InputSourceOTLP)
		b.config.Processors.NamespaceFilters[processorID] = filterByNamespaceProcessorConfig(pipeline.Spec.Input.OTLP.Namespaces, otlpInputSource())
	}
}

func (b *Builder) addConnectors(pipelineName string) {
	forwardConnectorID := formatForwardConnectorID(pipelineName)
	b.config.Connectors[forwardConnectorID] = struct{}{}

	routingConnectorID := formatRoutingConnectorID(pipelineName)
	b.config.Connectors[routingConnectorID] = routingConnectorConfig(pipelineName)
}

func (b *Builder) addOTLPExporter(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline, queueSize int) error {
	otlpExporterBuilder := otlpexporter.NewConfigBuilder(
		b.Reader,
		pipeline.Spec.Output.OTLP,
		pipeline.Name,
		queueSize,
		otlpexporter.SignalTypeMetric,
	)

	otlpExporterConfig, otlpExporterEnvVars, err := otlpExporterBuilder.MakeConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to make otlp exporter config: %w", err)
	}

	maps.Copy(b.envVars, otlpExporterEnvVars)

	exporterID := otlpexporter.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
	b.config.Exporters[exporterID] = Exporter{OTLP: otlpExporterConfig}

	return nil
}

func shouldFilterByNamespace(namespaceSelector *telemetryv1alpha1.NamespaceSelector) bool {
	return namespaceSelector != nil && (len(namespaceSelector.Include) > 0 || len(namespaceSelector.Exclude) > 0)
}
