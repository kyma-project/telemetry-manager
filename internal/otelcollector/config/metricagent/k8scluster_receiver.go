package metricagent

import (
	"time"
)

func k8sClusterReceiver(runtimeResources runtimeResourceSources, additionalMetrics []string, collectionInterval time.Duration) *K8sClusterReceiverConfig {
	return &K8sClusterReceiverConfig{
		AuthType:               "serviceAccount",
		CollectionInterval:     collectionInterval.String(),
		NodeConditionsToReport: []string{},
		K8sLeaderElector:       "k8s_leader_elector",
		Metrics:                k8sClusterMetrics(runtimeResources, additionalMetrics),
	}
}

func k8sClusterMetrics(runtimeResources runtimeResourceSources, additionalMetrics []string) K8sClusterMetrics {
	metrics := K8sClusterMetrics{}

	disableK8sClusterMetrics(&metrics, runtimeResources)
	enableK8sClusterAdditionalMetrics(&metrics, additionalMetrics)

	return metrics
}

func disableK8sClusterMetrics(metrics *K8sClusterMetrics, runtimeResources runtimeResourceSources) {
	//nolint:dupl // repeating the code as we want to test the metrics are disabled correctly
	metrics.K8sClusterDefaultMetricsToDrop = &K8sClusterDefaultMetricsToDrop{
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
		K8sCronJobActiveJobs:                Metric{Enabled: false},
	}

	// The following metrics are enabled by default in the K8sClusterReceiver. If we disable these resources in
	// pipeline config we need to disable the corresponding metrics in the K8sClusterReceiver.

	if !runtimeResources.pod {
		metrics.K8sClusterPodMetricsToDrop = &K8sClusterPodMetricsToDrop{
			K8sPodPhase: Metric{false},
		}
	}

	if !runtimeResources.container {
		metrics.K8sClusterContainerMetricsToDrop = &K8sClusterContainerMetricsToDrop{
			K8sContainerCPURequest:    Metric{false},
			K8sContainerCPULimit:      Metric{false},
			K8sContainerMemoryRequest: Metric{false},
			K8sContainerMemoryLimit:   Metric{false},
			K8sContainerRestarts:      Metric{false},
		}
	}

	if !runtimeResources.statefulset {
		metrics.K8sClusterStatefulSetMetricsToDrop = &K8sClusterStatefulSetMetricsToDrop{
			K8sStatefulSetCurrentPods: Metric{false},
			K8sStatefulSetDesiredPods: Metric{false},
			K8sStatefulSetReadyPods:   Metric{false},
			K8sStatefulSetUpdatedPods: Metric{false},
		}
	}

	if !runtimeResources.job {
		metrics.K8sClusterJobMetricsToDrop = &K8sClusterJobMetricsToDrop{
			K8sJobActivePods:            Metric{false},
			K8sJobDesiredSuccessfulPods: Metric{false},
			K8sJobFailedPods:            Metric{false},
			K8sJobMaxParallelPods:       Metric{false},
			K8sJobSuccessfulPods:        Metric{false},
		}
	}

	if !runtimeResources.deployment {
		metrics.K8sClusterDeploymentMetricsToDrop = &K8sClusterDeploymentMetricsToDrop{
			K8sDeploymentAvailable: Metric{false},
			K8sDeploymentDesired:   Metric{false},
		}
	}

	if !runtimeResources.daemonset {
		metrics.K8sClusterDaemonSetMetricsToDrop = &K8sClusterDaemonSetMetricsToDrop{
			K8sDaemonSetCurrentScheduledNodes: Metric{false},
			K8sDaemonSetDesiredScheduledNodes: Metric{false},
			K8sDaemonSetMisscheduledNodes:     Metric{false},
			K8sDaemonSetReadyNodes:            Metric{false},
		}
	}
}

func enableK8sClusterAdditionalMetrics(metrics *K8sClusterMetrics, additionalMetrics []string) {
	for _, m := range additionalMetrics {
		switch m {
		// K8sClusterDefaultMetricsToDrop
		case metricK8sContainerStorageRequest:
			metrics.K8sContainerStorageRequest.Enabled = true
		case metricK8sContainerStorageLimit:
			metrics.K8sContainerStorageLimit.Enabled = true
		case metricK8sContainerEphemeralStorageReq:
			metrics.K8sContainerEphemeralStorageRequest.Enabled = true
		case metricK8sContainerEphemeralStorageLimit:
			metrics.K8sContainerEphemeralStorageLimit.Enabled = true
		case metricK8sContainerReady:
			metrics.K8sContainerReady.Enabled = true
		case metricK8sNamespacePhase:
			metrics.K8sNamespacePhase.Enabled = true
		case metricK8sHPACurrentReplicas:
			metrics.K8sHPACurrentReplicas.Enabled = true
		case metricK8sHPADesiredReplicas:
			metrics.K8sHPADesiredReplicas.Enabled = true
		case metricK8sHPAMinReplicas:
			metrics.K8sHPAMinReplicas.Enabled = true
		case metricK8sHPAMaxReplicas:
			metrics.K8sHPAMaxReplicas.Enabled = true
		case metricK8sReplicaSetAvailable:
			metrics.K8sReplicaSetAvailable.Enabled = true
		case metricK8sReplicaSetDesired:
			metrics.K8sReplicaSetDesired.Enabled = true
		case metricK8sReplicationControllerAvailable:
			metrics.K8sReplicationControllerAvailable.Enabled = true
		case metricK8sReplicationControllerDesired:
			metrics.K8sReplicationControllerDesired.Enabled = true
		case metricK8sResourceQuotaHardLimit:
			metrics.K8sResourceQuotaHardLimit.Enabled = true
		case metricK8sResourceQuotaUsed:
			metrics.K8sResourceQuotaUsed.Enabled = true
		case metricK8sCronJobActiveJobs:
			metrics.K8sCronJobActiveJobs.Enabled = true

		// K8sClusterPodMetricsToDrop
		case metricK8sPodPhase:
			initPodMetrics(metrics)
			metrics.K8sPodPhase.Enabled = true

		// K8sClusterContainerMetricsToDrop
		case metricK8sContainerCPURequest:
			initContainerMetrics(metrics)
			metrics.K8sContainerCPURequest.Enabled = true
		case metricK8sContainerCPULimit:
			initContainerMetrics(metrics)
			metrics.K8sContainerCPULimit.Enabled = true
		case metricK8sContainerMemoryRequest:
			initContainerMetrics(metrics)
			metrics.K8sContainerMemoryRequest.Enabled = true
		case metricK8sContainerMemoryLimit:
			initContainerMetrics(metrics)
			metrics.K8sContainerMemoryLimit.Enabled = true
		case metricK8sContainerRestarts:
			initContainerMetrics(metrics)
			metrics.K8sContainerRestarts.Enabled = true

		// K8sClusterStatefulSetMetricsToDrop
		case metricK8sStatefulSetCurrentPods:
			initStatefulSetMetrics(metrics)
			metrics.K8sStatefulSetCurrentPods.Enabled = true
		case metricK8sStatefulSetDesiredPods:
			initStatefulSetMetrics(metrics)
			metrics.K8sStatefulSetDesiredPods.Enabled = true
		case metricK8sStatefulSetReadyPods:
			initStatefulSetMetrics(metrics)
			metrics.K8sStatefulSetReadyPods.Enabled = true
		case metricK8sStatefulSetUpdatedPods:
			initStatefulSetMetrics(metrics)
			metrics.K8sStatefulSetUpdatedPods.Enabled = true

		// K8sClusterJobMetricsToDrop
		case metricK8sJobActivePods:
			initJobMetrics(metrics)
			metrics.K8sJobActivePods.Enabled = true
		case metricK8sJobDesiredSuccessfulPods:
			initJobMetrics(metrics)
			metrics.K8sJobDesiredSuccessfulPods.Enabled = true
		case metricK8sJobFailedPods:
			initJobMetrics(metrics)
			metrics.K8sJobFailedPods.Enabled = true
		case metricK8sJobMaxParallelPods:
			initJobMetrics(metrics)
			metrics.K8sJobMaxParallelPods.Enabled = true
		case metricK8sJobSuccessfulPods:
			initJobMetrics(metrics)
			metrics.K8sJobSuccessfulPods.Enabled = true

		// K8sClusterDeploymentMetricsToDrop
		case metricK8sDeploymentAvailable:
			initDeploymentMetrics(metrics)
			metrics.K8sDeploymentAvailable.Enabled = true
		case metricK8sDeploymentDesired:
			initDeploymentMetrics(metrics)
			metrics.K8sDeploymentDesired.Enabled = true

		// K8sClusterDaemonSetMetricsToDrop
		case metricK8sDaemonSetCurrentScheduledNodes:
			initDaemonSetMetrics(metrics)
			metrics.K8sDaemonSetCurrentScheduledNodes.Enabled = true
		case metricK8sDaemonSetDesiredScheduledNodes:
			initDaemonSetMetrics(metrics)
			metrics.K8sDaemonSetDesiredScheduledNodes.Enabled = true
		case metricK8sDaemonSetMisscheduledNodes:
			initDaemonSetMetrics(metrics)
			metrics.K8sDaemonSetMisscheduledNodes.Enabled = true
		case metricK8sDaemonSetReadyNodes:
			initDaemonSetMetrics(metrics)
			metrics.K8sDaemonSetReadyNodes.Enabled = true

		// K8sClusterOptionalMetrics
		case metricK8sContainerStatusReason:
			initOptionalMetrics(metrics)
			metrics.K8sContainerStatusReason.Enabled = true
		case metricK8sContainerStatusState:
			initOptionalMetrics(metrics)
			metrics.K8sContainerStatusState.Enabled = true
		case metricK8sNodeCondition:
			initOptionalMetrics(metrics)
			metrics.K8sNodeCondition.Enabled = true
		case metricK8sPodStatusReason:
			initOptionalMetrics(metrics)
			metrics.K8sPodStatusReason.Enabled = true
		case metricK8sServiceEndpointCount:
			initOptionalMetrics(metrics)
			metrics.K8sServiceEndpointCount.Enabled = true
		case metricK8sServiceLBIngressCount:
			initOptionalMetrics(metrics)
			metrics.K8sServiceLoadBalancerIngressCount.Enabled = true
		}
	}
}

func initPodMetrics(metrics *K8sClusterMetrics) {
	if metrics.K8sClusterPodMetricsToDrop == nil {
		metrics.K8sClusterPodMetricsToDrop = &K8sClusterPodMetricsToDrop{}
	}
}

func initContainerMetrics(metrics *K8sClusterMetrics) {
	if metrics.K8sClusterContainerMetricsToDrop == nil {
		metrics.K8sClusterContainerMetricsToDrop = &K8sClusterContainerMetricsToDrop{}
	}
}

func initStatefulSetMetrics(metrics *K8sClusterMetrics) {
	if metrics.K8sClusterStatefulSetMetricsToDrop == nil {
		metrics.K8sClusterStatefulSetMetricsToDrop = &K8sClusterStatefulSetMetricsToDrop{}
	}
}

func initJobMetrics(metrics *K8sClusterMetrics) {
	if metrics.K8sClusterJobMetricsToDrop == nil {
		metrics.K8sClusterJobMetricsToDrop = &K8sClusterJobMetricsToDrop{}
	}
}

func initDeploymentMetrics(metrics *K8sClusterMetrics) {
	if metrics.K8sClusterDeploymentMetricsToDrop == nil {
		metrics.K8sClusterDeploymentMetricsToDrop = &K8sClusterDeploymentMetricsToDrop{}
	}
}

func initDaemonSetMetrics(metrics *K8sClusterMetrics) {
	if metrics.K8sClusterDaemonSetMetricsToDrop == nil {
		metrics.K8sClusterDaemonSetMetricsToDrop = &K8sClusterDaemonSetMetricsToDrop{}
	}
}

func initOptionalMetrics(metrics *K8sClusterMetrics) {
	if metrics.K8sClusterOptionalMetrics == nil {
		metrics.K8sClusterOptionalMetrics = &K8sClusterOptionalMetrics{}
	}
}
