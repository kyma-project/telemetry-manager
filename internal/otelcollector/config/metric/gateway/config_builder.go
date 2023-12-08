package gateway

import (
	"context"
	"fmt"
	"maps"
	"sort"

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

		otlpExporterBuilder := otlpexporter.NewConfigBuilder(c, pipeline.Spec.Output.Otlp, pipeline.Name, queueSize)
		if err := addComponentsForMetricPipeline(ctx, otlpExporterBuilder, &pipeline, cfg, envVars); err != nil {
			return nil, nil, err
		}
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

// addComponentsForMetricPipeline enriches a Config (exporters, processors, etc.) with components for a given telemetryv1alpha1.MetricPipeline.
func addComponentsForMetricPipeline(ctx context.Context, otlpExporterBuilder *otlpexporter.ConfigBuilder, pipeline *telemetryv1alpha1.MetricPipeline, cfg *Config, envVars otlpexporter.EnvVars) error {
	input := pipeline.Spec.Input
	if !input.Runtime.Enabled {
		cfg.Processors.DropIfInputSourceRuntime = makeDropIfInputSourceRuntimeConfig()
	}
	if !input.Prometheus.Enabled {
		cfg.Processors.DropIfInputSourcePrometheus = makeDropIfInputSourcePrometheusConfig()
	}
	if !input.Istio.Enabled {
		cfg.Processors.DropIfInputSourceIstio = makeDropIfInputSourceIstioConfig()
	}
	if !input.Otlp.Enabled {
		cfg.Processors.DropIfInputSourceOtlp = makeDropIfInputSourceOtlpConfig()
	}

	if cfg.Processors.NamespaceFilters == nil {
		cfg.Processors.NamespaceFilters = make(NamespaceFilters)
	}

	if shouldFilterByNamespace(input.Runtime.Enabled, input.Runtime.Namespaces) {
		processorName := getNamespaceFilterProcessorName(pipeline.Name, metric.InputSourceRuntime)
		cfg.Processors.NamespaceFilters[processorName] = makeFilterByNamespaceRuntimeInputConfig(pipeline.Spec.Input.Runtime.Namespaces)
	}
	if shouldFilterByNamespace(input.Prometheus.Enabled, input.Prometheus.Namespaces) {
		processorName := getNamespaceFilterProcessorName(pipeline.Name, metric.InputSourcePrometheus)
		cfg.Processors.NamespaceFilters[processorName] = makeFilterByNamespacePrometheusInputConfig(pipeline.Spec.Input.Prometheus.Namespaces)
	}
	if shouldFilterByNamespace(input.Istio.Enabled, input.Istio.Namespaces) {
		processorName := getNamespaceFilterProcessorName(pipeline.Name, metric.InputSourceIstio)
		cfg.Processors.NamespaceFilters[processorName] = makeFilterByNamespaceIstioInputConfig(pipeline.Spec.Input.Istio.Namespaces)
	}
	if shouldFilterByNamespace(input.Otlp.Enabled, input.Otlp.Namespaces) {
		processorName := getNamespaceFilterProcessorName(pipeline.Name, metric.InputSourceOtlp)
		cfg.Processors.NamespaceFilters[processorName] = makeFilterByNamespaceOtlpInputConfig(pipeline.Spec.Input.Otlp.Namespaces)
	}

	otlpExporterConfig, otlpExporterEnvVars, err := otlpExporterBuilder.MakeConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to make otlp exporter config: %w", err)
	}

	maps.Copy(envVars, otlpExporterEnvVars)

	otlpExporterID := otlpexporter.ExporterID(pipeline.Spec.Output.Otlp, pipeline.Name)
	cfg.Exporters[otlpExporterID] = Exporter{OTLP: otlpExporterConfig}

	pipelineID := fmt.Sprintf("metrics/%s", pipeline.Name)
	cfg.Service.Pipelines[pipelineID] = makePipelineConfig(pipeline, otlpExporterID)

	return nil
}

func makePipelineConfig(pipeline *telemetryv1alpha1.MetricPipeline, exporterIDs ...string) config.Pipeline {
	sort.Strings(exporterIDs)

	processors := []string{"memory_limiter", "k8sattributes"}

	input := pipeline.Spec.Input
	if !input.Runtime.Enabled {
		processors = append(processors, "filter/drop-if-input-source-runtime")
	}
	if !input.Prometheus.Enabled {
		processors = append(processors, "filter/drop-if-input-source-prometheus")
	}
	if !input.Istio.Enabled {
		processors = append(processors, "filter/drop-if-input-source-istio")
	}
	if !input.Otlp.Enabled {
		processors = append(processors, "filter/drop-if-input-source-otlp")
	}

	if shouldFilterByNamespace(input.Runtime.Enabled, input.Runtime.Namespaces) {
		processorName := getNamespaceFilterProcessorName(pipeline.Name, metric.InputSourceRuntime)
		processors = append(processors, processorName)
	}
	if shouldFilterByNamespace(input.Prometheus.Enabled, input.Prometheus.Namespaces) {
		processorName := getNamespaceFilterProcessorName(pipeline.Name, metric.InputSourcePrometheus)
		processors = append(processors, processorName)
	}
	if shouldFilterByNamespace(input.Istio.Enabled, input.Istio.Namespaces) {
		processorName := getNamespaceFilterProcessorName(pipeline.Name, metric.InputSourceIstio)
		processors = append(processors, processorName)
	}
	if shouldFilterByNamespace(input.Otlp.Enabled, input.Otlp.Namespaces) {
		processorName := getNamespaceFilterProcessorName(pipeline.Name, metric.InputSourceOtlp)
		processors = append(processors, processorName)
	}

	processors = append(processors, "resource/insert-cluster-name", "transform/resolve-service-name", "resource/drop-kyma-attributes", "batch")

	return config.Pipeline{
		Receivers:  []string{"otlp"},
		Processors: processors,
		Exporters:  exporterIDs,
	}
}

func shouldFilterByNamespace(enabled bool, namespaceSelector telemetryv1alpha1.MetricPipelineInputNamespaceSelector) bool {
	return enabled && (len(namespaceSelector.Include) > 0 || len(namespaceSelector.Exclude) > 0 || !*namespaceSelector.System)
}

func getNamespaceFilterProcessorName(pipelineName string, inputSourceType metric.InputSourceType) string {
	return fmt.Sprintf("filter/%s-filter-by-namespace-%s-input", pipelineName, inputSourceType)
}
