package gateway

import (
	"context"
	"fmt"
	"sort"

	"golang.org/x/exp/maps"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/common/otlpexporter"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

type otlpExporterConfigBuilder struct {
	ctx       context.Context
	c         client.Reader
	pipeline  *telemetryv1alpha1.MetricPipeline
	queueSize int
}

func (b *otlpExporterConfigBuilder) build() (*common.OTLPExporterConfig, otlpexporter.EnvVars, error) {
	return otlpexporter.MakeExporterConfig(b.ctx, b.c, b.pipeline.Spec.Output.Otlp, b.pipeline.Name, b.queueSize)
}

func MakeConfig(ctx context.Context, c client.Reader, pipelines []telemetryv1alpha1.MetricPipeline) (*Config, otlpexporter.EnvVars, error) {
	config := &Config{
		BaseConfig: common.BaseConfig{
			Service:    makeServiceConfig(),
			Extensions: makeExtensionsConfig(),
		},
		Receivers:  makeReceiversConfig(),
		Processors: makeProcessorsConfig(),
		Exporters:  make(ExportersConfig),
	}

	envVars := make(otlpexporter.EnvVars)
	queueSize := 256 / len(pipelines)

	for i := range pipelines {
		pipeline := pipelines[i]
		if pipeline.DeletionTimestamp != nil {
			continue
		}

		if err := addComponentsForMetricPipeline(otlpExporterConfigBuilder{
			ctx:       ctx,
			c:         c,
			pipeline:  &pipeline,
			queueSize: queueSize,
		}, &pipeline, config, envVars); err != nil {
			return nil, nil, err
		}
	}

	return config, envVars, nil
}

func makeReceiversConfig() ReceiversConfig {
	return ReceiversConfig{
		OTLP: common.OTLPReceiverConfig{
			Protocols: common.ReceiverProtocols{
				HTTP: common.EndpointConfig{
					Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.OTLPHTTP),
				},
				GRPC: common.EndpointConfig{
					Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.OTLPGRPC),
				},
			},
		},
	}
}

func makeExtensionsConfig() common.ExtensionsConfig {
	return common.ExtensionsConfig{
		HealthCheck: common.EndpointConfig{
			Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.HealthCheck),
		},
		Pprof: common.EndpointConfig{
			Endpoint: fmt.Sprintf("127.0.0.1:%d", ports.Pprof),
		},
	}
}

func makeServiceConfig() common.ServiceConfig {
	return common.ServiceConfig{
		Pipelines: make(common.PipelinesConfig),
		Telemetry: common.TelemetryConfig{
			Metrics: common.MetricsConfig{
				Address: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.Metrics),
			},
			Logs: common.LoggingConfig{
				Level: "info",
			},
		},
		Extensions: []string{"health_check", "pprof"},
	}
}

// addComponentsForMetricPipeline enriches a Config (exporters, processors, etc.) with components for a given MetricPipeline.
func addComponentsForMetricPipeline(otlpOutputBuilder otlpExporterConfigBuilder, pipeline *telemetryv1alpha1.MetricPipeline, config *Config, envVars otlpexporter.EnvVars) error {
	if enableDropIfInputSourceRuntime(pipeline) {
		config.Processors.DropIfInputSourceRuntime = makeDropIfInputSourceRuntimeConfig()
	}

	if enableDropIfInputSourceWorkloads(pipeline) {
		config.Processors.DropIfInputSourceWorkloads = makeDropIfInputSourceWorkloadsConfig()
	}

	otlpExporterConfig, otlpExporterEnvVars, err := otlpOutputBuilder.build()
	if err != nil {
		return fmt.Errorf("failed to make otlp exporter config: %w", err)
	}

	maps.Copy(envVars, otlpExporterEnvVars)

	otlpExporterID := otlpexporter.ExporterID(pipeline.Spec.Output.Otlp, pipeline.Name)
	config.Exporters[otlpExporterID] = ExporterConfig{OTLP: otlpExporterConfig}

	loggingExporterID := fmt.Sprintf("logging/%s", pipeline.Name)
	config.Exporters[loggingExporterID] = ExporterConfig{Logging: &common.LoggingExporterConfig{Verbosity: "basic"}}

	pipelineID := fmt.Sprintf("metrics/%s", pipeline.Name)
	config.Service.Pipelines[pipelineID] = makePipelineConfig(pipeline, otlpExporterID, loggingExporterID)

	return nil
}

func makePipelineConfig(pipeline *telemetryv1alpha1.MetricPipeline, exporterIDs ...string) common.PipelineConfig {
	sort.Strings(exporterIDs)

	processors := []string{"memory_limiter", "k8sattributes", "resource"}

	if enableDropIfInputSourceRuntime(pipeline) {
		processors = append(processors, "filter/drop-if-input-source-runtime")
	}

	if enableDropIfInputSourceWorkloads(pipeline) {
		processors = append(processors, "filter/drop-if-input-source-workloads")
	}

	processors = append(processors, "batch")

	return common.PipelineConfig{
		Receivers:  []string{"otlp"},
		Processors: processors,
		Exporters:  exporterIDs,
	}
}

func enableDropIfInputSourceRuntime(pipeline *telemetryv1alpha1.MetricPipeline) bool {
	appInput := pipeline.Spec.Input.Application
	return !appInput.Runtime.Enabled
}

func enableDropIfInputSourceWorkloads(pipeline *telemetryv1alpha1.MetricPipeline) bool {
	appInput := pipeline.Spec.Input.Application
	return !appInput.Workloads.Enabled
}
