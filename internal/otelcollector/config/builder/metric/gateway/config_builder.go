package gateway

import (
	"context"
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/otlpoutput"
)

func MakeConfig(ctx context.Context, c client.Reader, pipelines []v1alpha1.MetricPipeline) (*Config, otlpoutput.EnvVars, error) {
	exportersConfig, pipelinesConfig, allVars, err := makeExportersConfig(ctx, c, pipelines)
	if err != nil {
		return nil, nil, err
	}

	return &Config{
		Receivers:  makeReceiversConfig(),
		Processors: makeProcessorsConfig(),
		Exporters:  exportersConfig,
		Service:    makeServiceConfig(pipelinesConfig),
		Extensions: makeExtensionsConfig(),
	}, allVars, nil
}

func makeReceiversConfig() ReceiversConfig {
	return ReceiversConfig{
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
