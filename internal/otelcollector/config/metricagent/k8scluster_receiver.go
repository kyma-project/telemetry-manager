package metricagent

import (
	"time"
)

func k8sClusterReceiver(runtimeResources runtimeResourceSources, collectionInterval time.Duration) *K8sClusterReceiverConfig {
	return &K8sClusterReceiverConfig{
		AuthType:               "serviceAccount",
		CollectionInterval:     collectionInterval.String(),
		NodeConditionsToReport: []string{},
		K8sLeaderElector:       "k8s_leader_elector",
		Metrics:                k8sClusterMetricsToDrop(runtimeResources),
	}
}

func k8sClusterMetricsToDrop(runtimeResources runtimeResourceSources) K8sClusterMetrics {
	metricsToDrop := K8sClusterMetrics{}

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
