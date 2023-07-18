package gateway

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/otlpoutput"
)

func MakeConfig(ctx context.Context, c client.Reader, pipelines []v1alpha1.MetricPipeline) (*Config, otlpoutput.EnvVars, error) {
	exportersConfig, pipelinesConfig, allVars, err := makeExportersConfig(ctx, c, pipelines)
	if err != nil {
		return nil, nil, err
	}

	return &Config{
		BaseConfig: common.BaseConfig{
			Service:    makeServiceConfig(pipelinesConfig),
			Extensions: makeExtensionsConfig(),
		},
		Receivers:  makeReceiversConfig(),
		Processors: makeProcessorsConfig(),
		Exporters:  exportersConfig,
	}, allVars, nil
}

func makeReceiversConfig() ReceiversConfig {
	return ReceiversConfig{
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
