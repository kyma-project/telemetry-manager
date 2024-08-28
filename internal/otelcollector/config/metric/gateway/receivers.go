package gateway

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
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
		LeaderElection: LeaderElection{
			LeaseName:      "telemetry-metric-gateway-kymastats",
			LeaseNamespace: gatewayNamespace,
		},
		SingletonKymaStatsReceiver: SingletonKymaStatsReceiver{
			KymaStatsReceiver: KymaStatsReceiver{
				AuthType:           "serviceAccount",
				CollectionInterval: "30s",
				Modules: []ModuleGVR{
					{
						Group:    "operator.kyma-project.io",
						Version:  "v1alpha1",
						Resource: "telemetries",
					},
				},
			},
		},
	}
}

func makeSingletonK8sClusterReceiverCreatorConfig(gatewayNamespace string) *SingletonK8sClusterReceiverCreator {
	metricsToDrop := K8sClusterMetricsConfig{
		K8sContainerStorageRequest:          MetricConfig{false},
		K8sContainerStorageLimit:            MetricConfig{false},
		K8sContainerEphemeralStorageRequest: MetricConfig{false},
		K8sContainerEphemeralStorageLimit:   MetricConfig{false},
		K8sContainerRestarts:                MetricConfig{false},
		K8sContainerReady:                   MetricConfig{false},
		K8sNamespacePhase:                   MetricConfig{false},
		K8sReplicationControllerAvailable:   MetricConfig{false},
		K8sReplicationControllerDesired:     MetricConfig{false},
	}

	return &SingletonK8sClusterReceiverCreator{
		AuthType: "serviceAccount",
		LeaderElection: LeaderElection{
			LeaseName:      "telemetry-metric-gateway-k8scluster",
			LeaseNamespace: gatewayNamespace,
		},
		SingletonK8sClusterReceiver: SingletonK8sClusterReceiver{
			K8sClusterReceiver: K8sClusterReceiver{
				AuthType:               "serviceAccount",
				CollectionInterval:     "30s",
				NodeConditionsToReport: []string{},
				Metrics:                metricsToDrop,
			},
		},
	}
}
