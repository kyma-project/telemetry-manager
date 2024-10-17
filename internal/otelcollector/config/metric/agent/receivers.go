package agent

import (
	"fmt"
	"time"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
)

func makeReceiversConfig(inputs inputSources, opts BuildOptions) Receivers {
	var receiversConfig Receivers

	if inputs.prometheus {
		receiversConfig.PrometheusAppPods = makePrometheusConfigForPods(opts)
		receiversConfig.PrometheusAppServices = makePrometheusConfigForServices(opts)
	}

	if inputs.runtime {
		receiversConfig.KubeletStats = makeKubeletStatsConfig(inputs.runtimeResources)
		receiversConfig.SingletonK8sClusterReceiverCreator = makeSingletonK8sClusterReceiverCreatorConfig(opts.AgentNamespace)
	}

	if inputs.istio {
		receiversConfig.PrometheusIstio = makePrometheusIstioConfig()
	}

	return receiversConfig
}

func makeKubeletStatsConfig(runtimeResources runtimeResourcesEnabled) *KubeletStatsReceiver {
	const (
		collectionInterval = "30s"
		portKubelet        = 10250
	)

	return &KubeletStatsReceiver{
		CollectionInterval: collectionInterval,
		AuthType:           "serviceAccount",
		InsecureSkipVerify: true,
		Endpoint:           fmt.Sprintf("https://${%s}:%d", config.EnvVarCurrentNodeName, portKubelet),
		MetricGroups:       makeKubeletStatsMetricGroups(runtimeResources),
		Metrics: KubeletStatsMetricsConfig{
			ContainerCPUUsage:            MetricConfig{Enabled: true},
			ContainerCPUUtilization:      MetricConfig{Enabled: false},
			K8sPodCPUUsage:               MetricConfig{Enabled: true},
			K8sPodCPUUtilization:         MetricConfig{Enabled: false},
			K8sNodeCPUUsage:              MetricConfig{Enabled: true},
			K8sNodeCPUUtilization:        MetricConfig{Enabled: false},
			K8sNodeCPUTime:               MetricConfig{Enabled: false},
			K8sNodeMemoryMajorPageFaults: MetricConfig{Enabled: false},
			K8sNodeMemoryPageFaults:      MetricConfig{Enabled: false},
			K8sNodeNetworkIO:             MetricConfig{Enabled: false},
			K8sNodeNetworkErrors:         MetricConfig{Enabled: false},
		},
		ExtraMetadataLabels: []string{"k8s.volume.type"},
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
		LeaderElection: metric.LeaderElection{
			LeaseName:      "telemetry-metric-agent-k8scluster",
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

func makeKubeletStatsMetricGroups(runtimeResources runtimeResourcesEnabled) []MetricGroupType {
	var metricGroups []MetricGroupType

	if runtimeResources.container {
		metricGroups = append(metricGroups, MetricGroupTypeContainer)
	}

	if runtimeResources.pod {
		metricGroups = append(metricGroups, MetricGroupTypePod)
	}

	if runtimeResources.node {
		metricGroups = append(metricGroups, MetricGroupTypeNode)
	}

	if runtimeResources.volume {
		metricGroups = append(metricGroups, MetricGroupTypeVolume)
	}

	return metricGroups
}
