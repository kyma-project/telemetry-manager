package metricagent

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
)

func kubeletStatsReceiver(runtimeResources runtimeResourceSources) *KubeletStatsReceiverConfig {
	const (
		collectionInterval = "30s"
		portKubelet        = 10250
	)

	return &KubeletStatsReceiverConfig{
		CollectionInterval: collectionInterval,
		AuthType:           "serviceAccount",
		InsecureSkipVerify: true,
		Endpoint:           fmt.Sprintf("https://${%s}:%d", common.EnvVarCurrentNodeName, portKubelet),
		MetricGroups:       kubeletStatsMetricGroups(runtimeResources),
		Metrics: KubeletStatsMetrics{
			ContainerCPUUsage:            Metric{Enabled: true},
			K8sPodCPUUsage:               Metric{Enabled: true},
			K8sNodeCPUUsage:              Metric{Enabled: true},
			K8sNodeCPUTime:               Metric{Enabled: false},
			K8sNodeMemoryMajorPageFaults: Metric{Enabled: false},
			K8sNodeMemoryPageFaults:      Metric{Enabled: false},
		},
		// These resource attributes have been deprecated by OTel and will be removed in future versions.
		// The volume types associated with them have already been removed for the K8S versions that we use (v1.28+).
		// See: https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/45896
		ResourceAttributes: KubeletStatsResourceAttributes{
			AWSVolumeID:            Metric{Enabled: false},
			FSType:                 Metric{Enabled: false},
			GCEPDName:              Metric{Enabled: false},
			GlusterFSEndpointsName: Metric{Enabled: false},
			GlusterFSPath:          Metric{Enabled: false},
			Partition:              Metric{Enabled: false},
		},
		ExtraMetadataLabels: []string{"k8s.volume.type"},
		CollectAllNetworkInterfaces: NetworkInterfacesEnabler{
			NodeMetrics: true,
		},
	}
}

func k8sClusterReceiver(runtimeResources runtimeResourceSources) *K8sClusterReceiverConfig {
	return &K8sClusterReceiverConfig{
		AuthType:               "serviceAccount",
		CollectionInterval:     "30s",
		NodeConditionsToReport: []string{},
		K8sLeaderElector:       "k8s_leader_elector",
		Metrics:                k8sClusterMetricsToDrop(runtimeResources),
	}
}

func k8sClusterMetricsToDrop(runtimeResources runtimeResourceSources) K8sClusterMetricsToDrop {
	metricsToDrop := K8sClusterMetricsToDrop{}

	//nolint:dupl // repeating the code as we want to test the metrics are disabled correctly
	metricsToDrop.K8sClusterDefaultMetricsToDrop = &K8sClusterDefaultMetricsToDrop{
		K8sContainerStorageRequest:          Metric{Enabled: false},
		K8sContainerStorageLimit:            Metric{Enabled: false},
		K8sContainerEphemeralStorageRequest: Metric{Enabled: false},
		K8sContainerEphemeralStorageLimit:   Metric{Enabled: false},
		K8sContainerReady:                   Metric{Enabled: false},
		K8sNamespacePhase:                   Metric{Enabled: false},
		K8sHPACurrentReplicas:               Metric{Enabled: false},
		K8sHPADesiredReplicas:               Metric{Enabled: false},
		K8sHPAMinReplicas:                   Metric{Enabled: false},
		K8sHPAMaxReplicas:                   Metric{Enabled: false},
		K8sReplicaSetAvailable:              Metric{Enabled: false},
		K8sReplicaSetDesired:                Metric{Enabled: false},
		K8sReplicationControllerAvailable:   Metric{Enabled: false},
		K8sReplicationControllerDesired:     Metric{Enabled: false},
		K8sResourceQuotaHardLimit:           Metric{Enabled: false},
		K8sResourceQuotaUsed:                Metric{Enabled: false},
	}

	// The following metrics are enabled by default in the K8sClusterReceiver. If we disable these resources in
	// pipeline config we need to disable the corresponding metrics in the K8sClusterReceiver.

	if !runtimeResources.pod {
		metricsToDrop.K8sClusterPodMetricsToDrop = &K8sClusterPodMetricsToDrop{
			K8sPodPhase: Metric{false},
		}
	}

	if !runtimeResources.container {
		metricsToDrop.K8sClusterContainerMetricsToDrop = &K8sClusterContainerMetricsToDrop{
			K8sContainerCPURequest:    Metric{false},
			K8sContainerCPULimit:      Metric{false},
			K8sContainerMemoryRequest: Metric{false},
			K8sContainerMemoryLimit:   Metric{false},
			K8sContainerRestarts:      Metric{false},
		}
	}

	if !runtimeResources.statefulset {
		metricsToDrop.K8sClusterStatefulSetMetricsToDrop = &K8sClusterStatefulSetMetricsToDrop{
			K8sStatefulSetCurrentPods: Metric{false},
			K8sStatefulSetDesiredPods: Metric{false},
			K8sStatefulSetReadyPods:   Metric{false},
			K8sStatefulSetUpdatedPods: Metric{false},
		}
	}

	if !runtimeResources.job {
		metricsToDrop.K8sClusterJobMetricsToDrop = &K8sClusterJobMetricsToDrop{
			K8sJobActivePods:            Metric{false},
			K8sJobDesiredSuccessfulPods: Metric{false},
			K8sJobFailedPods:            Metric{false},
			K8sJobMaxParallelPods:       Metric{false},
		}
	}

	if !runtimeResources.deployment {
		metricsToDrop.K8sClusterDeploymentMetricsToDrop = &K8sClusterDeploymentMetricsToDrop{
			K8sDeploymentAvailable: Metric{false},
			K8sDeploymentDesired:   Metric{false},
		}
	}

	if !runtimeResources.daemonset {
		metricsToDrop.K8sClusterDaemonSetMetricsToDrop = &K8sClusterDaemonSetMetricsToDrop{
			K8sDaemonSetCurrentScheduledNodes: Metric{false},
			K8sDaemonSetDesiredScheduledNodes: Metric{false},
			K8sDaemonSetMisscheduledNodes:     Metric{false},
			K8sDaemonSetReadyNodes:            Metric{false},
		}
	}

	return metricsToDrop
}

func kubeletStatsMetricGroups(runtimeResources runtimeResourceSources) []MetricGroupType {
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
