package metricgateway

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

func receiversConfig() Receivers {
	return Receivers{
		KymaStatsReceiver: kymaStatsReceiverConfig(),
		OTLP: common.OTLPReceiver{
			Protocols: common.ReceiverProtocols{
				HTTP: common.Endpoint{
					Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.OTLPHTTP),
				},
				GRPC: common.Endpoint{
					Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.OTLPGRPC),
				},
			},
		},
	}
}

func kymaStatsReceiverConfig() *KymaStatsReceiver {
	return &KymaStatsReceiver{
		AuthType:           "serviceAccount",
		K8sLeaderElector:   "k8s_leader_elector",
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
	}
}
