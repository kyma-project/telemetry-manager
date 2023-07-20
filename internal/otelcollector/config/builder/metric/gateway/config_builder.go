package gateway

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/otlpoutput"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
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

func makeServiceConfig(pipelines common.PipelinesConfig) common.ServiceConfig {
	return common.ServiceConfig{
		Pipelines: pipelines,
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
