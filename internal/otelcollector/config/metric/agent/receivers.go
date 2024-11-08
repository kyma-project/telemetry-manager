package agent

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
)

func makeReceiversConfig(inputs inputSources, opts BuildOptions) Receivers {
	var receiversConfig Receivers

	if inputs.prometheus {
		receiversConfig.PrometheusAppPods = makePrometheusConfigForPods()
		receiversConfig.PrometheusAppServices = makePrometheusConfigForServices(opts)
	}

	if inputs.runtime {
		receiversConfig.KubeletStats = makeKubeletStatsConfig(inputs.runtimeResources)
		receiversConfig.SingletonK8sClusterReceiverCreator = makeSingletonK8sClusterReceiverCreatorConfig(opts.AgentNamespace, inputs.runtimeResources)
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

func makeSingletonK8sClusterReceiverCreatorConfig(gatewayNamespace string, runtimeResources runtimeResourcesEnabled) *SingletonK8sClusterReceiverCreator {
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
				Metrics:                makeK8sClusterMetricsToDrop(runtimeResources),
			},
		},
	}
}

func makeK8sClusterMetricsToDrop(runtimeResources runtimeResourcesEnabled) K8sClusterMetricsToDrop {
	metricsToDrop := K8sClusterMetricsToDrop{}

	//nolint:dupl // repeating the code as we want to test the metrics are disabled correctly
	metricsToDrop.K8sClusterDefaultMetricsToDrop = &K8sClusterDefaultMetricsToDrop{
		K8sContainerStorageRequest:          MetricConfig{Enabled: false},
		K8sContainerStorageLimit:            MetricConfig{Enabled: false},
		K8sContainerEphemeralStorageRequest: MetricConfig{Enabled: false},
		K8sContainerEphemeralStorageLimit:   MetricConfig{Enabled: false},
		K8sContainerRestarts:                MetricConfig{Enabled: false},
		K8sContainerReady:                   MetricConfig{Enabled: false},
		K8sNamespacePhase:                   MetricConfig{Enabled: false},
		K8sHPACurrentReplicas:               MetricConfig{Enabled: false},
		K8sHPADesiredReplicas:               MetricConfig{Enabled: false},
		K8sHPAMinReplicas:                   MetricConfig{Enabled: false},
		K8sHPAMaxReplicas:                   MetricConfig{Enabled: false},
		K8sReplicaSetAvailable:              MetricConfig{Enabled: false},
		K8sReplicaSetDesired:                MetricConfig{Enabled: false},
		K8sReplicationControllerAvailable:   MetricConfig{Enabled: false},
		K8sReplicationControllerDesired:     MetricConfig{Enabled: false},
		K8sResourceQuotaHardLimit:           MetricConfig{Enabled: false},
		K8sResourceQuotaUsed:                MetricConfig{Enabled: false},
	}

	// The following metrics are enabled by default in the K8sClusterReceiver. If we disable these resources in
	// pipeline config we need to disable the corresponding metrics in the K8sClusterReceiver.

	if !runtimeResources.pod {
		metricsToDrop.K8sClusterPodMetricsToDrop = &K8sClusterPodMetricsToDrop{
			K8sPodPhase: MetricConfig{false},
		}
	}

	if !runtimeResources.container {
		metricsToDrop.K8sClusterContainerMetricsToDrop = &K8sClusterContainerMetricsToDrop{
			K8sContainerCPURequest:    MetricConfig{false},
			K8sContainerCPULimit:      MetricConfig{false},
			K8sContainerMemoryRequest: MetricConfig{false},
			K8sContainerMemoryLimit:   MetricConfig{false},
		}
	}

	if !runtimeResources.statefulset {
		metricsToDrop.K8sClusterStatefulSetMetricsToDrop = &K8sClusterStatefulSetMetricsToDrop{
			K8sStatefulSetCurrentPods: MetricConfig{false},
			K8sStatefulSetDesiredPods: MetricConfig{false},
			K8sStatefulSetReadyPods:   MetricConfig{false},
			K8sStatefulSetUpdatedPods: MetricConfig{false},
		}
	}

	if !runtimeResources.job {
		metricsToDrop.K8sClusterJobMetricsToDrop = &K8sClusterJobMetricsToDrop{
			K8sJobActivePods:            MetricConfig{false},
			K8sJobDesiredSuccessfulPods: MetricConfig{false},
			K8sJobFailedPods:            MetricConfig{false},
			K8sJobMaxParallelPods:       MetricConfig{false},
		}
	}

	if !runtimeResources.deployment {
		metricsToDrop.K8sClusterDeploymentMetricsToDrop = &K8sClusterDeploymentMetricsToDrop{
			K8sDeploymentAvailable: MetricConfig{false},
			K8sDeploymentDesired:   MetricConfig{false},
		}
	}

	if !runtimeResources.daemonset {
		metricsToDrop.K8sClusterDaemonSetMetricsToDrop = &K8sClusterDaemonSetMetricsToDrop{
			K8sDaemonSetCurrentScheduledNodes: MetricConfig{false},
			K8sDaemonSetDesiredScheduledNodes: MetricConfig{false},
			K8sDaemonSetMisscheduledNodes:     MetricConfig{false},
			K8sDaemonSetReadyNodes:            MetricConfig{false},
		}
	}

	return metricsToDrop
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
