package gateway

import (
	"context"
	"fmt"
	"sort"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/otlpoutput"
)

func MakeConfig(ctx context.Context, c client.Reader, pipelines []v1alpha1.TracePipeline) (*Config, otlpoutput.EnvVars, error) {
	allVars := make(otlpoutput.EnvVars)
	exportersConfig := make(ExportersConfig)
	pipelinesConfig := make(common.PipelinesConfig)

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
			exportersConfig[k] = ExporterConfig{BaseGatewayExporterConfig: v}
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

	return &Config{
		BaseConfig: common.BaseConfig{
			Service:    makeServiceConfig(pipelinesConfig),
			Extensions: makeExtensionsConfig(),
		},
		Exporters:  exportersConfig,
		Receivers:  makeReceiversConfig(),
		Processors: makeProcessorsConfig(),
	}, allVars, nil
}

func makeReceiversConfig() ReceiversConfig {
	return ReceiversConfig{
		OpenCensus: common.EndpointConfig{
			Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, common.PortOpenCensus),
		},
		OTLP: common.OTLPReceiverConfig{
			Protocols: common.ReceiverProtocols{
				HTTP: common.EndpointConfig{
					Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, common.PortOTLPHTTP),
				},
				GRPC: common.EndpointConfig{
					Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, common.PortOTLPGRPC),
				},
			},
		},
	}
}

func makePipelineConfig(outputAliases []string) common.PipelineConfig {
	return common.PipelineConfig{
		Receivers:  []string{"opencensus", "otlp"},
		Processors: []string{"memory_limiter", "k8sattributes", "filter", "resource", "batch"},
		Exporters:  outputAliases,
	}
}

func makeExtensionsConfig() common.ExtensionsConfig {
	return common.ExtensionsConfig{
		HealthCheck: common.EndpointConfig{
			Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, common.PortHealthCheck),
		},
		Pprof: common.EndpointConfig{
			Endpoint: fmt.Sprintf("127.0.0.1:%d", common.PortPprof),
		},
	}
}

func makeServiceConfig(pipelines common.PipelinesConfig) common.ServiceConfig {
	return common.ServiceConfig{
		Pipelines: pipelines,
		Telemetry: common.TelemetryConfig{
			Metrics: common.MetricsConfig{
				Address: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, common.PortMetrics),
			},
			Logs: common.LoggingConfig{
				Level: "info",
			},
		},
		Extensions: []string{"health_check", "pprof"},
	}
}
