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
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

func MakeConfig(ctx context.Context, c client.Reader, pipelines []telemetryv1alpha1.MetricPipeline) (*Config, otlpexporter.EnvVars, error) {
	cfg := &Config{
		Base: config.Base{
			Service:    makeServiceConfig(),
			Extensions: makeExtensionsConfig(),
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

		otlpExporterBuilder := otlpexporter.NewConfigBuilder(c, pipeline.Spec.Output.Otlp, pipeline.Name, queueSize, otlpexporter.SignalTypeMetric)
		if err := declareComponentsForMetricPipeline(ctx, otlpExporterBuilder, &pipeline, cfg, envVars); err != nil {
			return nil, nil, err
		}

		pipelineID := fmt.Sprintf("metrics/%s", pipeline.Name)
		cfg.Service.Pipelines[pipelineID] = makeServicePipelineConfig(&pipeline)
	}

	return cfg, envVars, nil
}

func makeReceiversConfig() Receivers {
	return Receivers{
		OTLP: config.OTLPReceiver{
			Protocols: config.ReceiverProtocols{
				HTTP: config.Endpoint{
					Endpoint: fmt.Sprintf("${%s}:%d", config.EnvVarCurrentPodIP, ports.OTLPHTTP),
				},
				GRPC: config.Endpoint{
					Endpoint: fmt.Sprintf("${%s}:%d", config.EnvVarCurrentPodIP, ports.OTLPGRPC),
				},
			},
		},
	}
}

func makeExtensionsConfig() config.Extensions {
	return config.Extensions{
		HealthCheck: config.Endpoint{
			Endpoint: fmt.Sprintf("${%s}:%d", config.EnvVarCurrentPodIP, ports.HealthCheck),
		},
		Pprof: config.Endpoint{
			Endpoint: fmt.Sprintf("127.0.0.1:%d", ports.Pprof),
		},
	}
}

func makeServiceConfig() config.Service {
	return config.Service{
		Pipelines: make(config.Pipelines),
		Telemetry: config.Telemetry{
			Metrics: config.Metrics{
				Address: fmt.Sprintf("${%s}:%d", config.EnvVarCurrentPodIP, ports.Metrics),
			},
			Logs: config.Logs{
				Level:    "info",
				Encoding: "json",
			},
		},
		Extensions: []string{"health_check", "pprof"},
	}
}

// declareComponentsForMetricPipeline enriches a Config (exporters, processors, etc.) with components for a given telemetryv1alpha1.MetricPipeline.
func declareComponentsForMetricPipeline(ctx context.Context, otlpExporterBuilder *otlpexporter.ConfigBuilder, pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config, envVars otlpexporter.EnvVars) error {
	declareDiagnosticMetricsDropFilters(pipeline, cfg)
	declareDropFilters(pipeline, cfg)
	declareNamespaceFilters(pipeline, cfg)
	return declareOTLPExporter(ctx, otlpExporterBuilder, pipeline, cfg, envVars)
}

func declareDropFilters(pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config) {
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

func declareDiagnosticMetricsDropFilters(pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config) {
	input := pipeline.Spec.Input

	if isPrometheusInputEnabled(input) && !isPrometheusDiagnosticMetricsEnabled(input) {
		cfg.Processors.DropDiagnosticMetricsIfInputSourcePrometheus = makeDropDiagnosticMetricsForInput(inputSourceEquals(metric.InputSourcePrometheus))
	}
	if isIstioInputEnabled(input) && !isIstioDiagnosticMetricsEnabled(input) {
		cfg.Processors.DropDiagnosticMetricsIfInputSourceIstio = makeDropDiagnosticMetricsForInput(inputSourceEquals(metric.InputSourceIstio))
	}
}

func declareNamespaceFilters(pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config) {
	if cfg.Processors.NamespaceFilters == nil {
		cfg.Processors.NamespaceFilters = make(NamespaceFilters)
	}

	input := pipeline.Spec.Input
	if isRuntimeInputEnabled(input) && shouldFilterByNamespace(input.Runtime.Namespaces) {
		processorID := makeNamespaceFilterID(pipeline.Name, metric.InputSourceRuntime)
		cfg.Processors.NamespaceFilters[processorID] = makeFilterByNamespaceRuntimeInputConfig(pipeline.Spec.Input.Runtime.Namespaces)
	}
	if isPrometheusInputEnabled(input) && shouldFilterByNamespace(input.Prometheus.Namespaces) {
		processorID := makeNamespaceFilterID(pipeline.Name, metric.InputSourcePrometheus)
		cfg.Processors.NamespaceFilters[processorID] = makeFilterByNamespacePrometheusInputConfig(pipeline.Spec.Input.Prometheus.Namespaces)
	}
	if isIstioInputEnabled(input) && shouldFilterByNamespace(input.Istio.Namespaces) {
		processorID := makeNamespaceFilterID(pipeline.Name, metric.InputSourceIstio)
		cfg.Processors.NamespaceFilters[processorID] = makeFilterByNamespaceIstioInputConfig(pipeline.Spec.Input.Istio.Namespaces)
	}
	if isOtlpInputEnabled(input) && input.Otlp != nil && shouldFilterByNamespace(input.Otlp.Namespaces) {
		processorID := makeNamespaceFilterID(pipeline.Name, metric.InputSourceOtlp)
		cfg.Processors.NamespaceFilters[processorID] = makeFilterByNamespaceOtlpInputConfig(pipeline.Spec.Input.Otlp.Namespaces)
	}
}

func declareOTLPExporter(ctx context.Context, otlpExporterBuilder *otlpexporter.ConfigBuilder, pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config, envVars otlpexporter.EnvVars) error {
	otlpExporterConfig, otlpExporterEnvVars, err := otlpExporterBuilder.MakeConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to make otlp exporter config: %w", err)
	}

	maps.Copy(envVars, otlpExporterEnvVars)

	exporterID := otlpexporter.ExporterID(pipeline.Spec.Output.Otlp, pipeline.Name)
	cfg.Exporters[exporterID] = Exporter{OTLP: otlpExporterConfig}

	return nil
}

func makeServicePipelineConfig(pipeline *telemetryv1alpha1.MetricPipeline) config.Pipeline {
	processors := []string{"memory_limiter", "k8sattributes"}

	input := pipeline.Spec.Input
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

	if isRuntimeInputEnabled(input) && shouldFilterByNamespace(input.Runtime.Namespaces) {
		processors = append(processors, makeNamespaceFilterID(pipeline.Name, metric.InputSourceRuntime))
	}
	if isPrometheusInputEnabled(input) && shouldFilterByNamespace(input.Prometheus.Namespaces) {
		processors = append(processors, makeNamespaceFilterID(pipeline.Name, metric.InputSourcePrometheus))
	}
	if isIstioInputEnabled(input) && shouldFilterByNamespace(input.Istio.Namespaces) {
		processors = append(processors, makeNamespaceFilterID(pipeline.Name, metric.InputSourceIstio))
	}
	if isOtlpInputEnabled(input) && input.Otlp != nil && shouldFilterByNamespace(input.Otlp.Namespaces) {
		processors = append(processors, makeNamespaceFilterID(pipeline.Name, metric.InputSourceOtlp))
	}

	processors = append(processors, makeDiagnosticMetricFilters(input)...)

	processors = append(processors, "resource/insert-cluster-name", "transform/resolve-service-name", "resource/drop-kyma-attributes", "batch")

	return config.Pipeline{
		Receivers:  []string{"otlp"},
		Processors: processors,
		Exporters:  []string{makeOTLPExporterID(pipeline)},
	}
}

func makeDiagnosticMetricFilters(input telemetryv1alpha1.MetricPipelineInput) []string {
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

func makeNamespaceFilterID(pipelineName string, inputSourceType metric.InputSourceType) string {
	return fmt.Sprintf("filter/%s-filter-by-namespace-%s-input", pipelineName, inputSourceType)
}

func makeOTLPExporterID(pipeline *telemetryv1alpha1.MetricPipeline) string {
	return otlpexporter.ExporterID(pipeline.Spec.Output.Otlp, pipeline.Name)
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
