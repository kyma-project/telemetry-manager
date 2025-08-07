package gateway

import (
	"context"
	"fmt"
	"maps"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/ottlexpr"
	metricpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/metricpipeline"
)

// newConfig constructs a global, pipeline-independent Base config for the metric gateway collector.
// It sets up default service and extension components, and returns a Config with initialized fields.
func newConfig(opts BuildOptions) *Config {
	return &Config{
		Base: config.DefaultBaseConfig(make(config.Pipelines),
			config.WithK8sLeaderElector("serviceAccount", "telemetry-metric-gateway-kymastats", opts.GatewayNamespace),
		),
		Receivers:  receiversConfig(),
		Processors: processorsConfig(opts),
		Exporters:  make(Exporters),
		Connectors: make(Connectors),
	}
}

// addMetricPipelineComponents enriches a Config (exporters, processors, etc.) with components for a given telemetryv1alpha1.MetricPipeline.
func (cfg *Config) addMetricPipelineComponents(
	ctx context.Context,
	otlpExporterBuilder *otlpexporter.ConfigBuilder,
	pipeline *telemetryv1alpha1.MetricPipeline,
	envVars otlpexporter.EnvVars,
) error {
	cfg.addDiagnosticMetricsDropFilters(pipeline)
	cfg.addInputSourceFilters(pipeline)
	cfg.addRuntimeResourcesFilters(pipeline)
	cfg.addNamespaceFilters(pipeline)
	cfg.addConnectors(pipeline.Name)

	return cfg.addOTLPExporter(ctx, otlpExporterBuilder, pipeline, envVars)
}

func (cfg *Config) addDiagnosticMetricsDropFilters(pipeline *telemetryv1alpha1.MetricPipeline) {
	input := pipeline.Spec.Input

	if metricpipelineutils.IsPrometheusInputEnabled(input) && !metricpipelineutils.IsPrometheusDiagnosticInputEnabled(input) {
		cfg.Processors.DropDiagnosticMetricsIfInputSourcePrometheus = dropDiagnosticMetricsFilterConfig(inputSourceEquals(metric.InputSourcePrometheus))
	}

	if metricpipelineutils.IsIstioInputEnabled(input) && !metricpipelineutils.IsIstioDiagnosticInputEnabled(input) {
		cfg.Processors.DropDiagnosticMetricsIfInputSourceIstio = dropDiagnosticMetricsFilterConfig(inputSourceEquals(metric.InputSourceIstio))
	}
}

func (cfg *Config) addInputSourceFilters(pipeline *telemetryv1alpha1.MetricPipeline) {
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

func (cfg *Config) addRuntimeResourcesFilters(pipeline *telemetryv1alpha1.MetricPipeline) {
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

func (cfg *Config) addNamespaceFilters(pipeline *telemetryv1alpha1.MetricPipeline) {
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

func (cfg *Config) addConnectors(pipelineName string) {
	forwardConnectorID := formatForwardConnectorID(pipelineName)
	cfg.Connectors[forwardConnectorID] = struct{}{}

	routingConnectorID := formatRoutingConnectorID(pipelineName)
	cfg.Connectors[routingConnectorID] = routingConnectorConfig(pipelineName)
}

func (cfg *Config) addOTLPExporter(ctx context.Context, otlpExporterBuilder *otlpexporter.ConfigBuilder, pipeline *telemetryv1alpha1.MetricPipeline, envVars otlpexporter.EnvVars) error {
	otlpExporterConfig, otlpExporterEnvVars, err := otlpExporterBuilder.MakeConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to make otlp exporter config: %w", err)
	}

	maps.Copy(envVars, otlpExporterEnvVars)

	exporterID := otlpexporter.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
	cfg.Exporters[exporterID] = Exporter{OTLP: otlpExporterConfig}

	return nil
}

func (cfg *Config) addInputPipeline(pipelineName string) {
	cfg.Service.Pipelines[formatInputPipelineID(pipelineName)] = config.Pipeline{
		Receivers:  []string{"otlp", "kymastats"},
		Processors: []string{"memory_limiter"},
		Exporters:  []string{formatRoutingConnectorID(pipelineName)},
	}
}

func (cfg *Config) addEnrichmentPipeline(pipelineName string) {
	cfg.Service.Pipelines[formatAttributesEnrichmentPipelineID(pipelineName)] = config.Pipeline{
		Receivers:  []string{formatRoutingConnectorID(pipelineName)},
		Processors: []string{"k8sattributes", "service_enrichment"},
		Exporters:  []string{formatForwardConnectorID(pipelineName)},
	}
}

func (cfg *Config) addOutputPipeline(pipeline *telemetryv1alpha1.MetricPipeline) {
	input := pipeline.Spec.Input

	processors := []string{}
	processors = append(processors, "transform/set-instrumentation-scope-kyma")
	processors = append(processors, inputSourceFiltersIDs(input)...)
	processors = append(processors, namespaceFiltersIDs(input, pipeline)...)
	processors = append(processors, runtimeResourcesFiltersIDs(input)...)
	processors = append(processors, diagnosticMetricFiltersIDs(input)...)
	processors = append(processors, "resource/insert-cluster-attributes", "resource/delete-skip-enrichment-attribute", "resource/drop-kyma-attributes", "batch")

	cfg.Service.Pipelines[formatOutputPipelineID(pipeline.Name)] = config.Pipeline{
		Receivers:  []string{formatRoutingConnectorID(pipeline.Name), formatForwardConnectorID(pipeline.Name)},
		Processors: processors,
		Exporters:  []string{formatOTLPExporterID(pipeline)},
	}
}

type Config struct {
	config.Base `yaml:",inline"`

	Receivers  Receivers  `yaml:"receivers"`
	Processors Processors `yaml:"processors"`
	Exporters  Exporters  `yaml:"exporters"`
	Connectors Connectors `yaml:"connectors"`
}

type Receivers struct {
	OTLP              config.OTLPReceiver `yaml:"otlp"`
	KymaStatsReceiver *KymaStatsReceiver  `yaml:"kymastats,omitempty"`
}

type KymaStatsReceiver struct {
	AuthType           string      `yaml:"auth_type"`
	CollectionInterval string      `yaml:"collection_interval"`
	Resources          []ModuleGVR `yaml:"resources"`
	K8sLeaderElector   string      `yaml:"k8s_leader_elector"`
}

type MetricConfig struct {
	Enabled bool `yaml:"enabled"`
}

type ModuleGVR struct {
	Group    string `yaml:"group"`
	Version  string `yaml:"version"`
	Resource string `yaml:"resource"`
}

type Processors struct {
	config.BaseProcessors `yaml:",inline"`

	K8sAttributes                                *config.K8sAttributesProcessor     `yaml:"k8sattributes,omitempty"`
	InsertClusterAttributes                      *config.ResourceProcessor          `yaml:"resource/insert-cluster-attributes,omitempty"`
	DropDiagnosticMetricsIfInputSourcePrometheus *FilterProcessor                   `yaml:"filter/drop-diagnostic-metrics-if-input-source-prometheus,omitempty"`
	DropDiagnosticMetricsIfInputSourceIstio      *FilterProcessor                   `yaml:"filter/drop-diagnostic-metrics-if-input-source-istio,omitempty"`
	DropIfInputSourceRuntime                     *FilterProcessor                   `yaml:"filter/drop-if-input-source-runtime,omitempty"`
	DropIfInputSourcePrometheus                  *FilterProcessor                   `yaml:"filter/drop-if-input-source-prometheus,omitempty"`
	DropIfInputSourceIstio                       *FilterProcessor                   `yaml:"filter/drop-if-input-source-istio,omitempty"`
	DropIfEnvoyMetricsDisabled                   *FilterProcessor                   `yaml:"filter/drop-envoy-metrics-if-disabled,omitempty"`
	DropIfInputSourceOTLP                        *FilterProcessor                   `yaml:"filter/drop-if-input-source-otlp,omitempty"`
	DropRuntimePodMetrics                        *FilterProcessor                   `yaml:"filter/drop-runtime-pod-metrics,omitempty"`
	DropRuntimeContainerMetrics                  *FilterProcessor                   `yaml:"filter/drop-runtime-container-metrics,omitempty"`
	DropRuntimeNodeMetrics                       *FilterProcessor                   `yaml:"filter/drop-runtime-node-metrics,omitempty"`
	DropRuntimeVolumeMetrics                     *FilterProcessor                   `yaml:"filter/drop-runtime-volume-metrics,omitempty"`
	DropRuntimeDeploymentMetrics                 *FilterProcessor                   `yaml:"filter/drop-runtime-deployment-metrics,omitempty"`
	DropRuntimeStatefulSetMetrics                *FilterProcessor                   `yaml:"filter/drop-runtime-statefulset-metrics,omitempty"`
	DropRuntimeDaemonSetMetrics                  *FilterProcessor                   `yaml:"filter/drop-runtime-daemonset-metrics,omitempty"`
	DropRuntimeJobMetrics                        *FilterProcessor                   `yaml:"filter/drop-runtime-job-metrics,omitempty"`
	ResolveServiceName                           *config.ServiceEnrichmentProcessor `yaml:"service_enrichment,omitempty"`
	DropKymaAttributes                           *config.ResourceProcessor          `yaml:"resource/drop-kyma-attributes,omitempty"`
	SetInstrumentationScopeKyma                  *metric.TransformProcessor         `yaml:"transform/set-instrumentation-scope-kyma,omitempty"`
	DeleteSkipEnrichmentAttribute                *config.ResourceProcessor          `yaml:"resource/delete-skip-enrichment-attribute,omitempty"`

	// NamespaceFilters contains filter processors, which need different configurations per pipeline
	NamespaceFilters NamespaceFilters `yaml:",inline,omitempty"`
}

type NamespaceFilters map[string]*FilterProcessor

type FilterProcessor struct {
	Metrics FilterProcessorMetrics `yaml:"metrics"`
}

type FilterProcessorMetrics struct {
	Metric []string `yaml:"metric,omitempty"`
}

type Exporters map[string]Exporter

type Exporter struct {
	OTLP *config.OTLPExporter `yaml:",inline,omitempty"`
}

// Connectors is a map of connectors. The key is the name of the connector. The value is the connector configuration.
// We need to have a different connector per pipeline, so we need to have a map of connectors.
// The value needs to be "any" to satisfy different types of connectors.
type Connectors map[string]any

type RoutingConnector struct {
	DefaultPipelines []string                     `yaml:"default_pipelines"`
	ErrorMode        string                       `yaml:"error_mode"`
	Table            []RoutingConnectorTableEntry `yaml:"table"`
}

type RoutingConnectorTableEntry struct {
	Statement string   `yaml:"statement"`
	Pipelines []string `yaml:"pipelines"`
}
