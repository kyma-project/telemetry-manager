package gateway

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

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

func makeSingletonKymaStatsReceiverCreatorConfig(gatewayNamespace string) *SingletonKymaStatsReceiverCreator {
	return &SingletonKymaStatsReceiverCreator{
		AuthType: "serviceAccount",
		LeaderElection: metric.LeaderElection{
			LeaseName:      "telemetry-metric-gateway-kymastats",
			LeaseNamespace: gatewayNamespace,
		},
		SingletonKymaStatsReceiver: SingletonKymaStatsReceiver{
			KymaStatsReceiver: KymaStatsReceiver{
				AuthType:           "serviceAccount",
				CollectionInterval: "30s",
				Resources: []ModuleGVR{
					{
						Group:    "operator.kyma-project.io",
						Version:  "v1alpha1",
						Resource: "telemetries",
					},
					{
						Group:    "telemetry.kyma-project.io",
						Version:  "v1alpha1",
						Resource: "logpipelines",
					},
					{
						Group:    "telemetry.kyma-project.io",
						Version:  "v1alpha1",
						Resource: "metricpipelines",
					},
					{
						Group:    "telemetry.kyma-project.io",
						Version:  "v1alpha1",
						Resource: "tracepipelines",
					},
				},
			},
		},
	}
}
