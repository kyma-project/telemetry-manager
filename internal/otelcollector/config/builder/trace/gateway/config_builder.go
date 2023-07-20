package gateway

import (
	"context"
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/common/otlpexporter"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	"golang.org/x/exp/maps"
	"sort"
)

func MakeConfig(ctx context.Context, c client.Reader, pipelines []telemetryv1alpha1.TracePipeline) (*Config, otlpexporter.EnvVars, error) {
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

		otlpExporterBuilder := otlpexporter.NewConfigBuilder(c, pipeline.Spec.Output.Otlp, pipeline.Name, queueSize)
		if err := addComponentsForTracePipeline(ctx, otlpExporterBuilder, &pipeline, config, envVars); err != nil {
			return nil, nil, err
		}
	}

	return config, envVars, nil
}

func makeReceiversConfig() ReceiversConfig {
	return ReceiversConfig{
		OpenCensus: common.EndpointConfig{
			Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.OpenCensus),
		},
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

// addComponentsForTracePipeline enriches a Config (exporters, processors, etc.) with components for a given telemetryv1alpha1.TracePipeline.
func addComponentsForTracePipeline(ctx context.Context, otlpExporterBuilder *otlpexporter.ConfigBuilder, pipeline *telemetryv1alpha1.TracePipeline, config *Config, envVars otlpexporter.EnvVars) error {
	otlpExporterConfig, otlpExporterEnvVars, err := otlpExporterBuilder.MakeConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to make otlp exporter config: %w", err)
	}

	maps.Copy(envVars, otlpExporterEnvVars)

	otlpExporterID := otlpexporter.ExporterID(pipeline.Spec.Output.Otlp, pipeline.Name)
	config.Exporters[otlpExporterID] = ExporterConfig{OTLP: otlpExporterConfig}

	loggingExporterID := fmt.Sprintf("logging/%s", pipeline.Name)
	config.Exporters[loggingExporterID] = ExporterConfig{Logging: &common.LoggingExporterConfig{Verbosity: "basic"}}

	pipelineID := fmt.Sprintf("traces/%s", pipeline.Name)
	config.Service.Pipelines[pipelineID] = makePipelineConfig(pipeline, otlpExporterID, loggingExporterID)

	return nil
}

func makePipelineConfig(pipeline *telemetryv1alpha1.TracePipeline, exporterIDs ...string) common.PipelineConfig {
	sort.Strings(exporterIDs)

	return common.PipelineConfig{
		Receivers:  []string{"opencensus", "otlp"},
		Processors: []string{"memory_limiter", "k8sattributes", "filter", "resource", "batch"},
		Exporters:  exporterIDs,
	}
}
