package gateway

import (
	"context"
	"fmt"
	"maps"
	"sort"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

func MakeConfig(ctx context.Context, c client.Reader, pipelines []telemetryv1alpha1.TracePipeline) (*Config, otlpexporter.EnvVars, error) {
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

		otlpExporterBuilder := otlpexporter.NewConfigBuilder(c, pipeline.Spec.Output.Otlp, pipeline.Name, queueSize, otlpexporter.SignalTypeTrace)
		if err := addComponentsForTracePipeline(ctx, otlpExporterBuilder, &pipeline, cfg, envVars); err != nil {
			return nil, nil, err
		}
	}

	return cfg, envVars, nil
}

func makeReceiversConfig() Receivers {
	return Receivers{
		OpenCensus: config.Endpoint{
			Endpoint: fmt.Sprintf("${%s}:%d", config.EnvVarCurrentPodIP, ports.OpenCensus),
		},
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

// addComponentsForTracePipeline enriches a Config (exporters, processors, etc.) with components for a given telemetryv1alpha1.TracePipeline.
func addComponentsForTracePipeline(ctx context.Context, otlpExporterBuilder *otlpexporter.ConfigBuilder, pipeline *telemetryv1alpha1.TracePipeline, cfg *Config, envVars otlpexporter.EnvVars) error {
	otlpExporterConfig, otlpExporterEnvVars, err := otlpExporterBuilder.MakeConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to make otlp exporter config: %w", err)
	}

	maps.Copy(envVars, otlpExporterEnvVars)

	otlpExporterID := otlpexporter.ExporterID(pipeline.Spec.Output.Otlp, pipeline.Name)
	cfg.Exporters[otlpExporterID] = Exporter{OTLP: otlpExporterConfig}

	pipelineID := fmt.Sprintf("traces/%s", pipeline.Name)
	cfg.Service.Pipelines[pipelineID] = makePipelineConfig(otlpExporterID)

	return nil
}

func makePipelineConfig(exporterIDs ...string) config.Pipeline {
	sort.Strings(exporterIDs)

	return config.Pipeline{
		Receivers: []string{"opencensus", "otlp"},
		Processors: []string{"memory_limiter",
			"k8sattributes",
			"filter/drop-noisy-spans",
			"resource/insert-cluster-name",
			"transform/resolve-service-name",
			"resource/drop-kyma-attributes",
			"batch",
		},
		Exporters: exporterIDs,
	}
}
