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

	// The following metrics are enabled by default in the K8sClusterReceiver.
	// If the resource selectors are disabled, we need to disable the corresponding metrics in the K8sClusterReceiver.

	if !runtimeResources.pod {
		metrics.K8sClusterPodMetrics = &K8sClusterPodMetrics{
			K8sPodPhase: Metric{false},
		}
	}

	if !runtimeResources.container {
		metrics.K8sClusterContainerMetrics = &K8sClusterContainerMetrics{
			K8sContainerCPURequest:    Metric{false},
			K8sContainerCPULimit:      Metric{false},
			K8sContainerMemoryRequest: Metric{false},
			K8sContainerMemoryLimit:   Metric{false},
			K8sContainerRestarts:      Metric{false},
		}
	}

	if !runtimeResources.statefulset {
		metrics.K8sClusterStatefulSetMetrics = &K8sClusterStatefulSetMetrics{
			K8sStatefulSetCurrentPods: Metric{false},
			K8sStatefulSetDesiredPods: Metric{false},
			K8sStatefulSetReadyPods:   Metric{false},
			K8sStatefulSetUpdatedPods: Metric{false},
		}
	}

	if !runtimeResources.job {
		metrics.K8sClusterJobMetrics = &K8sClusterJobMetrics{
			K8sJobActivePods:            Metric{false},
			K8sJobDesiredSuccessfulPods: Metric{false},
			K8sJobFailedPods:            Metric{false},
			K8sJobMaxParallelPods:       Metric{false},
			K8sJobSuccessfulPods:        Metric{false},
		}
	}

	if !runtimeResources.deployment {
		metrics.K8sClusterDeploymentMetrics = &K8sClusterDeploymentMetrics{
			K8sDeploymentAvailable: Metric{false},
			K8sDeploymentDesired:   Metric{false},
		}
	}

	if !runtimeResources.daemonset {
		metrics.K8sClusterDaemonSetMetrics = &K8sClusterDaemonSetMetrics{
			K8sDaemonSetCurrentScheduledNodes: Metric{false},
			K8sDaemonSetDesiredScheduledNodes: Metric{false},
			K8sDaemonSetMisscheduledNodes:     Metric{false},
			K8sDaemonSetReadyNodes:            Metric{false},
		}
	}
}

func enableK8sClusterAdditionalMetrics(metrics *K8sClusterMetrics, additionalMetrics []string) {
	for _, m := range additionalMetrics {
		if enabler, ok := k8sClusterMetricEnablers[m]; ok {
			enabler(metrics)
		}
	}
}

var k8sClusterMetricEnablers = map[string]func(*K8sClusterMetrics){
	// K8sClusterDefaultMetricsToDrop
	metricK8sContainerStorageRequest: func(m *K8sClusterMetrics) {
		m.K8sContainerStorageRequest.Enabled = true
	},
	metricK8sContainerStorageLimit: func(m *K8sClusterMetrics) {
		m.K8sContainerStorageLimit.Enabled = true
	},
	metricK8sContainerEphemeralStorageRequest: func(m *K8sClusterMetrics) {
		m.K8sContainerEphemeralStorageRequest.Enabled = true
	},
	metricK8sContainerEphemeralStorageLimit: func(m *K8sClusterMetrics) {
		m.K8sContainerEphemeralStorageLimit.Enabled = true
	},
	metricK8sContainerReady: func(m *K8sClusterMetrics) {
		m.K8sContainerReady.Enabled = true
	},
	metricK8sNamespacePhase: func(m *K8sClusterMetrics) {
		m.K8sNamespacePhase.Enabled = true
	},
	metricK8sHPACurrentReplicas: func(m *K8sClusterMetrics) {
		m.K8sHPACurrentReplicas.Enabled = true
	},
	metricK8sHPADesiredReplicas: func(m *K8sClusterMetrics) {
		m.K8sHPADesiredReplicas.Enabled = true
	},
	metricK8sHPAMinReplicas: func(m *K8sClusterMetrics) {
		m.K8sHPAMinReplicas.Enabled = true
	},
	metricK8sHPAMaxReplicas: func(m *K8sClusterMetrics) {
		m.K8sHPAMaxReplicas.Enabled = true
	},
	metricK8sReplicaSetAvailable: func(m *K8sClusterMetrics) {
		m.K8sReplicaSetAvailable.Enabled = true
	},
	metricK8sReplicaSetDesired: func(m *K8sClusterMetrics) {
		m.K8sReplicaSetDesired.Enabled = true
	},
	metricK8sReplicationControllerAvailable: func(m *K8sClusterMetrics) {
		m.K8sReplicationControllerAvailable.Enabled = true
	},
	metricK8sReplicationControllerDesired: func(m *K8sClusterMetrics) {
		m.K8sReplicationControllerDesired.Enabled = true
	},
	metricK8sResourceQuotaHardLimit: func(m *K8sClusterMetrics) {
		m.K8sResourceQuotaHardLimit.Enabled = true
	},
	metricK8sResourceQuotaUsed: func(m *K8sClusterMetrics) {
		m.K8sResourceQuotaUsed.Enabled = true
	},
	metricK8sCronJobActiveJobs: func(m *K8sClusterMetrics) {
		m.K8sCronJobActiveJobs.Enabled = true
	},

	// K8sClusterPodMetrics
	metricK8sPodPhase: func(m *K8sClusterMetrics) {
		initPodMetrics(m)
		m.K8sPodPhase.Enabled = true
	},

	// K8sClusterContainerMetrics
	metricK8sContainerCPURequest: func(m *K8sClusterMetrics) {
		initContainerMetrics(m)
		m.K8sContainerCPURequest.Enabled = true
	},
	metricK8sContainerCPULimit: func(m *K8sClusterMetrics) {
		initContainerMetrics(m)
		m.K8sContainerCPULimit.Enabled = true
	},
	metricK8sContainerMemoryRequest: func(m *K8sClusterMetrics) {
		initContainerMetrics(m)
		m.K8sContainerMemoryRequest.Enabled = true
	},
	metricK8sContainerMemoryLimit: func(m *K8sClusterMetrics) {
		initContainerMetrics(m)
		m.K8sContainerMemoryLimit.Enabled = true
	},
	metricK8sContainerRestarts: func(m *K8sClusterMetrics) {
		initContainerMetrics(m)
		m.K8sContainerRestarts.Enabled = true
	},

	// K8sClusterStatefulSetMetrics
	metricK8sStatefulSetCurrentPods: func(m *K8sClusterMetrics) {
		initStatefulSetMetrics(m)
		m.K8sStatefulSetCurrentPods.Enabled = true
	},
	metricK8sStatefulSetDesiredPods: func(m *K8sClusterMetrics) {
		initStatefulSetMetrics(m)
		m.K8sStatefulSetDesiredPods.Enabled = true
	},
	metricK8sStatefulSetReadyPods: func(m *K8sClusterMetrics) {
		initStatefulSetMetrics(m)
		m.K8sStatefulSetReadyPods.Enabled = true
	},
	metricK8sStatefulSetUpdatedPods: func(m *K8sClusterMetrics) {
		initStatefulSetMetrics(m)
		m.K8sStatefulSetUpdatedPods.Enabled = true
	},

	// K8sClusterJobMetrics
	metricK8sJobActivePods: func(m *K8sClusterMetrics) {
		initJobMetrics(m)
		m.K8sJobActivePods.Enabled = true
	},
	metricK8sJobDesiredSuccessfulPods: func(m *K8sClusterMetrics) {
		initJobMetrics(m)
		m.K8sJobDesiredSuccessfulPods.Enabled = true
	},
	metricK8sJobFailedPods: func(m *K8sClusterMetrics) {
		initJobMetrics(m)
		m.K8sJobFailedPods.Enabled = true
	},
	metricK8sJobMaxParallelPods: func(m *K8sClusterMetrics) {
		initJobMetrics(m)
		m.K8sJobMaxParallelPods.Enabled = true
	},
	metricK8sJobSuccessfulPods: func(m *K8sClusterMetrics) {
		initJobMetrics(m)
		m.K8sJobSuccessfulPods.Enabled = true
	},

	// K8sClusterDeploymentMetrics
	metricK8sDeploymentAvailable: func(m *K8sClusterMetrics) {
		initDeploymentMetrics(m)
		m.K8sDeploymentAvailable.Enabled = true
	},
	metricK8sDeploymentDesired: func(m *K8sClusterMetrics) {
		initDeploymentMetrics(m)
		m.K8sDeploymentDesired.Enabled = true
	},

	// K8sClusterDaemonSetMetrics
	metricK8sDaemonSetCurrentScheduledNodes: func(m *K8sClusterMetrics) {
		initDaemonSetMetrics(m)
		m.K8sDaemonSetCurrentScheduledNodes.Enabled = true
	},
	metricK8sDaemonSetDesiredScheduledNodes: func(m *K8sClusterMetrics) {
		initDaemonSetMetrics(m)
		m.K8sDaemonSetDesiredScheduledNodes.Enabled = true
	},
	metricK8sDaemonSetMisscheduledNodes: func(m *K8sClusterMetrics) {
		initDaemonSetMetrics(m)
		m.K8sDaemonSetMisscheduledNodes.Enabled = true
	},
	metricK8sDaemonSetReadyNodes: func(m *K8sClusterMetrics) {
		initDaemonSetMetrics(m)
		m.K8sDaemonSetReadyNodes.Enabled = true
	},

	// K8sClusterOptionalMetrics
	metricK8sContainerStatusReason: func(m *K8sClusterMetrics) {
		initOptionalMetrics(m)
		m.K8sContainerStatusReason.Enabled = true
	},
	metricK8sContainerStatusState: func(m *K8sClusterMetrics) {
		initOptionalMetrics(m)
		m.K8sContainerStatusState.Enabled = true
	},
	metricK8sNodeCondition: func(m *K8sClusterMetrics) {
		initOptionalMetrics(m)
		m.K8sNodeCondition.Enabled = true
	},
	metricK8sPodStatusReason: func(m *K8sClusterMetrics) {
		initOptionalMetrics(m)
		m.K8sPodStatusReason.Enabled = true
	},
	metricK8sServiceEndpointCount: func(m *K8sClusterMetrics) {
		initOptionalMetrics(m)
		m.K8sServiceEndpointCount.Enabled = true
	},
	metricK8sServiceLBIngressCount: func(m *K8sClusterMetrics) {
		initOptionalMetrics(m)
		m.K8sServiceLoadBalancerIngressCount.Enabled = true
	},
}

func initPodMetrics(metrics *K8sClusterMetrics) {
	if metrics.K8sClusterPodMetrics == nil {
		metrics.K8sClusterPodMetrics = &K8sClusterPodMetrics{}
	}
}

func initContainerMetrics(metrics *K8sClusterMetrics) {
	if metrics.K8sClusterContainerMetrics == nil {
		metrics.K8sClusterContainerMetrics = &K8sClusterContainerMetrics{}
	}
}

func initStatefulSetMetrics(metrics *K8sClusterMetrics) {
	if metrics.K8sClusterStatefulSetMetrics == nil {
		metrics.K8sClusterStatefulSetMetrics = &K8sClusterStatefulSetMetrics{}
	}
}

func initJobMetrics(metrics *K8sClusterMetrics) {
	if metrics.K8sClusterJobMetrics == nil {
		metrics.K8sClusterJobMetrics = &K8sClusterJobMetrics{}
	}
}

func initDeploymentMetrics(metrics *K8sClusterMetrics) {
	if metrics.K8sClusterDeploymentMetrics == nil {
		metrics.K8sClusterDeploymentMetrics = &K8sClusterDeploymentMetrics{}
	}
}

func initDaemonSetMetrics(metrics *K8sClusterMetrics) {
	if metrics.K8sClusterDaemonSetMetrics == nil {
		metrics.K8sClusterDaemonSetMetrics = &K8sClusterDaemonSetMetrics{}
	}
}

func initOptionalMetrics(metrics *K8sClusterMetrics) {
	if metrics.K8sClusterOptionalMetrics == nil {
		metrics.K8sClusterOptionalMetrics = &K8sClusterOptionalMetrics{}
	}
}
