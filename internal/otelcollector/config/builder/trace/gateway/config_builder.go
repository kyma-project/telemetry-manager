package gateway

import (
	"context"
	"fmt"
	"sort"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/otlpoutput"
)

func MakeConfig(ctx context.Context, c client.Reader, pipelines []v1alpha1.TracePipeline) (*config.Config, otlpoutput.EnvVars, error) {
	allVars := make(otlpoutput.EnvVars)
	exportersConfig := make(config.ExportersConfig)
	pipelinesConfig := make(config.PipelinesConfig)

	for _, pipeline := range pipelines {
		if pipeline.DeletionTimestamp != nil {
			continue
		}

		output := pipeline.Spec.Output
		queueSize := 256 / len(pipelines)
		exporterConfig, envVars, err := otlpoutput.MakeExportersConfig(ctx, c, output.Otlp, pipeline.Name, queueSize)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to make exporter config: %v", err)
		}

		var outputAliases []string
		for k, v := range exporterConfig {
			exportersConfig[k] = v
			outputAliases = append(outputAliases, k)
		}
		sort.Strings(outputAliases)
		pipelineConfig := makePipelineConfig(outputAliases)
		pipelineName := fmt.Sprintf("traces/%s", pipeline.Name)
		pipelinesConfig[pipelineName] = pipelineConfig

		for k, v := range envVars {
			allVars[k] = v
		}
	}

	return &config.Config{
		Exporters:  exportersConfig,
		Receivers:  makeReceiversConfig(),
		Processors: makeProcessorsConfig(),
		Service:    makeServiceConfig(pipelinesConfig),
		Extensions: makeExtensionsConfig(),
	}, allVars, nil
}

func makeReceiversConfig() config.ReceiversConfig {
	return config.ReceiversConfig{
		OpenCensus: &config.EndpointConfig{
			Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, common.PortOpenCensus),
		},
		OTLP: &config.OTLPReceiverConfig{
			Protocols: config.ReceiverProtocols{
				HTTP: config.EndpointConfig{
					Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, common.PortOTLPHTTP),
				},
				GRPC: config.EndpointConfig{
					Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, common.PortOTLPGRPC),
				},
			},
		},
	}
}

func makePipelineConfig(outputAliases []string) config.PipelineConfig {
	return config.PipelineConfig{
		Receivers:  []string{"opencensus", "otlp"},
		Processors: []string{"memory_limiter", "k8sattributes", "filter", "resource", "batch"},
		Exporters:  outputAliases,
	}
}

func makeExtensionsConfig() config.ExtensionsConfig {
	return config.ExtensionsConfig{
		HealthCheck: config.EndpointConfig{
			Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, common.PortHealthCheck),
		},
		Pprof: config.EndpointConfig{
			Endpoint: fmt.Sprintf("127.0.0.1:%d", common.PortPprof),
		},
	}
}

func makeServiceConfig(pipelines config.PipelinesConfig) config.ServiceConfig {
	return config.ServiceConfig{
		Pipelines: pipelines,
		Telemetry: config.TelemetryConfig{
			Metrics: config.MetricsConfig{
				Address: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, common.PortMetrics),
			},
			Logs: config.LoggingConfig{
				Level: "info",
			},
		},
		Extensions: []string{"health_check", "pprof"},
	}
}
