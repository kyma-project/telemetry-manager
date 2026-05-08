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
		K8sContainerStorageRequest:          &Metric{Enabled: false},
		K8sContainerStorageLimit:            &Metric{Enabled: false},
		K8sContainerEphemeralStorageRequest: &Metric{Enabled: false},
		K8sContainerEphemeralStorageLimit:   &Metric{Enabled: false},
		K8sContainerReady:                   &Metric{Enabled: false},
		K8sNamespacePhase:                   &Metric{Enabled: false},
		K8sHPACurrentReplicas:               &Metric{Enabled: false},
		K8sHPADesiredReplicas:               &Metric{Enabled: false},
		K8sHPAMinReplicas:                   &Metric{Enabled: false},
		K8sHPAMaxReplicas:                   &Metric{Enabled: false},
		K8sReplicaSetAvailable:              &Metric{Enabled: false},
		K8sReplicaSetDesired:                &Metric{Enabled: false},
		K8sReplicationControllerAvailable:   &Metric{Enabled: false},
		K8sReplicationControllerDesired:     &Metric{Enabled: false},
		K8sResourceQuotaHardLimit:           &Metric{Enabled: false},
		K8sResourceQuotaUsed:                &Metric{Enabled: false},
		K8sCronJobActiveJobs:                &Metric{Enabled: false},
	}

	// The following metrics are enabled by default in the K8sClusterReceiver.
	// If the resource selectors are disabled, we need to disable the corresponding metrics in the K8sClusterReceiver.

	if !runtimeResources.pod {
		metrics.K8sClusterPodMetrics = &K8sClusterPodMetrics{
			K8sPodPhase: &Metric{Enabled: false},
		}
	}

	if !runtimeResources.container {
		metrics.K8sClusterContainerMetrics = &K8sClusterContainerMetrics{
			K8sContainerCPURequest:    &Metric{Enabled: false},
			K8sContainerCPULimit:      &Metric{Enabled: false},
			K8sContainerMemoryRequest: &Metric{Enabled: false},
			K8sContainerMemoryLimit:   &Metric{Enabled: false},
			K8sContainerRestarts:      &Metric{Enabled: false},
		}
	}

	if !runtimeResources.statefulset {
		metrics.K8sClusterStatefulSetMetrics = &K8sClusterStatefulSetMetrics{
			K8sStatefulSetCurrentPods: &Metric{Enabled: false},
			K8sStatefulSetDesiredPods: &Metric{Enabled: false},
			K8sStatefulSetReadyPods:   &Metric{Enabled: false},
			K8sStatefulSetUpdatedPods: &Metric{Enabled: false},
		}
	}

	if !runtimeResources.job {
		metrics.K8sClusterJobMetrics = &K8sClusterJobMetrics{
			K8sJobActivePods:            &Metric{Enabled: false},
			K8sJobDesiredSuccessfulPods: &Metric{Enabled: false},
			K8sJobFailedPods:            &Metric{Enabled: false},
			K8sJobMaxParallelPods:       &Metric{Enabled: false},
			K8sJobSuccessfulPods:        &Metric{Enabled: false},
		}
	}

	if !runtimeResources.deployment {
		metrics.K8sClusterDeploymentMetrics = &K8sClusterDeploymentMetrics{
			K8sDeploymentAvailable: &Metric{Enabled: false},
			K8sDeploymentDesired:   &Metric{Enabled: false},
		}
	}

	if !runtimeResources.daemonset {
		metrics.K8sClusterDaemonSetMetrics = &K8sClusterDaemonSetMetrics{
			K8sDaemonSetCurrentScheduledNodes: &Metric{Enabled: false},
			K8sDaemonSetDesiredScheduledNodes: &Metric{Enabled: false},
			K8sDaemonSetMisscheduledNodes:     &Metric{Enabled: false},
			K8sDaemonSetReadyNodes:            &Metric{Enabled: false},
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
		m.K8sContainerStorageRequest = &Metric{Enabled: true}
	},
	metricK8sContainerStorageLimit: func(m *K8sClusterMetrics) {
		m.K8sContainerStorageLimit = &Metric{Enabled: true}
	},
	metricK8sContainerEphemeralStorageRequest: func(m *K8sClusterMetrics) {
		m.K8sContainerEphemeralStorageRequest = &Metric{Enabled: true}
	},
	metricK8sContainerEphemeralStorageLimit: func(m *K8sClusterMetrics) {
		m.K8sContainerEphemeralStorageLimit = &Metric{Enabled: true}
	},
	metricK8sContainerReady: func(m *K8sClusterMetrics) {
		m.K8sContainerReady = &Metric{Enabled: true}
	},
	metricK8sNamespacePhase: func(m *K8sClusterMetrics) {
		m.K8sNamespacePhase = &Metric{Enabled: true}
	},
	metricK8sHPACurrentReplicas: func(m *K8sClusterMetrics) {
		m.K8sHPACurrentReplicas = &Metric{Enabled: true}
	},
	metricK8sHPADesiredReplicas: func(m *K8sClusterMetrics) {
		m.K8sHPADesiredReplicas = &Metric{Enabled: true}
	},
	metricK8sHPAMinReplicas: func(m *K8sClusterMetrics) {
		m.K8sHPAMinReplicas = &Metric{Enabled: true}
	},
	metricK8sHPAMaxReplicas: func(m *K8sClusterMetrics) {
		m.K8sHPAMaxReplicas = &Metric{Enabled: true}
	},
	metricK8sReplicaSetAvailable: func(m *K8sClusterMetrics) {
		m.K8sReplicaSetAvailable = &Metric{Enabled: true}
	},
	metricK8sReplicaSetDesired: func(m *K8sClusterMetrics) {
		m.K8sReplicaSetDesired = &Metric{Enabled: true}
	},
	metricK8sReplicationControllerAvailable: func(m *K8sClusterMetrics) {
		m.K8sReplicationControllerAvailable = &Metric{Enabled: true}
	},
	metricK8sReplicationControllerDesired: func(m *K8sClusterMetrics) {
		m.K8sReplicationControllerDesired = &Metric{Enabled: true}
	},
	metricK8sResourceQuotaHardLimit: func(m *K8sClusterMetrics) {
		m.K8sResourceQuotaHardLimit = &Metric{Enabled: true}
	},
	metricK8sResourceQuotaUsed: func(m *K8sClusterMetrics) {
		m.K8sResourceQuotaUsed = &Metric{Enabled: true}
	},
	metricK8sCronJobActiveJobs: func(m *K8sClusterMetrics) {
		m.K8sCronJobActiveJobs = &Metric{Enabled: true}
	},

	// K8sClusterPodMetrics
	metricK8sPodPhase: func(m *K8sClusterMetrics) {
		initPodMetrics(m)
		m.K8sPodPhase = &Metric{Enabled: true}
	},

	// K8sClusterContainerMetrics
	metricK8sContainerCPURequest: func(m *K8sClusterMetrics) {
		initContainerMetrics(m)
		m.K8sContainerCPURequest = &Metric{Enabled: true}
	},
	metricK8sContainerCPULimit: func(m *K8sClusterMetrics) {
		initContainerMetrics(m)
		m.K8sContainerCPULimit = &Metric{Enabled: true}
	},
	metricK8sContainerMemoryRequest: func(m *K8sClusterMetrics) {
		initContainerMetrics(m)
		m.K8sContainerMemoryRequest = &Metric{Enabled: true}
	},
	metricK8sContainerMemoryLimit: func(m *K8sClusterMetrics) {
		initContainerMetrics(m)
		m.K8sContainerMemoryLimit = &Metric{Enabled: true}
	},
	metricK8sContainerRestarts: func(m *K8sClusterMetrics) {
		initContainerMetrics(m)
		m.K8sContainerRestarts = &Metric{Enabled: true}
	},

	// K8sClusterStatefulSetMetrics
	metricK8sStatefulSetCurrentPods: func(m *K8sClusterMetrics) {
		initStatefulSetMetrics(m)
		m.K8sStatefulSetCurrentPods = &Metric{Enabled: true}
	},
	metricK8sStatefulSetDesiredPods: func(m *K8sClusterMetrics) {
		initStatefulSetMetrics(m)
		m.K8sStatefulSetDesiredPods = &Metric{Enabled: true}
	},
	metricK8sStatefulSetReadyPods: func(m *K8sClusterMetrics) {
		initStatefulSetMetrics(m)
		m.K8sStatefulSetReadyPods = &Metric{Enabled: true}
	},
	metricK8sStatefulSetUpdatedPods: func(m *K8sClusterMetrics) {
		initStatefulSetMetrics(m)
		m.K8sStatefulSetUpdatedPods = &Metric{Enabled: true}
	},

	// K8sClusterJobMetrics
	metricK8sJobActivePods: func(m *K8sClusterMetrics) {
		initJobMetrics(m)
		m.K8sJobActivePods = &Metric{Enabled: true}
	},
	metricK8sJobDesiredSuccessfulPods: func(m *K8sClusterMetrics) {
		initJobMetrics(m)
		m.K8sJobDesiredSuccessfulPods = &Metric{Enabled: true}
	},
	metricK8sJobFailedPods: func(m *K8sClusterMetrics) {
		initJobMetrics(m)
		m.K8sJobFailedPods = &Metric{Enabled: true}
	},
	metricK8sJobMaxParallelPods: func(m *K8sClusterMetrics) {
		initJobMetrics(m)
		m.K8sJobMaxParallelPods = &Metric{Enabled: true}
	},
	metricK8sJobSuccessfulPods: func(m *K8sClusterMetrics) {
		initJobMetrics(m)
		m.K8sJobSuccessfulPods = &Metric{Enabled: true}
	},

	// K8sClusterDeploymentMetrics
	metricK8sDeploymentAvailable: func(m *K8sClusterMetrics) {
		initDeploymentMetrics(m)
		m.K8sDeploymentAvailable = &Metric{Enabled: true}
	},
	metricK8sDeploymentDesired: func(m *K8sClusterMetrics) {
		initDeploymentMetrics(m)
		m.K8sDeploymentDesired = &Metric{Enabled: true}
	},

	// K8sClusterDaemonSetMetrics
	metricK8sDaemonSetCurrentScheduledNodes: func(m *K8sClusterMetrics) {
		initDaemonSetMetrics(m)
		m.K8sDaemonSetCurrentScheduledNodes = &Metric{Enabled: true}
	},
	metricK8sDaemonSetDesiredScheduledNodes: func(m *K8sClusterMetrics) {
		initDaemonSetMetrics(m)
		m.K8sDaemonSetDesiredScheduledNodes = &Metric{Enabled: true}
	},
	metricK8sDaemonSetMisscheduledNodes: func(m *K8sClusterMetrics) {
		initDaemonSetMetrics(m)
		m.K8sDaemonSetMisscheduledNodes = &Metric{Enabled: true}
	},
	metricK8sDaemonSetReadyNodes: func(m *K8sClusterMetrics) {
		initDaemonSetMetrics(m)
		m.K8sDaemonSetReadyNodes = &Metric{Enabled: true}
	},

	// K8sClusterOptionalMetrics
	metricK8sContainerStatusReason: func(m *K8sClusterMetrics) {
		initOptionalMetrics(m)
		m.K8sContainerStatusReason = &Metric{Enabled: true}
	},
	metricK8sContainerStatusState: func(m *K8sClusterMetrics) {
		initOptionalMetrics(m)
		m.K8sContainerStatusState = &Metric{Enabled: true}
	},
	metricK8sNodeCondition: func(m *K8sClusterMetrics) {
		initOptionalMetrics(m)
		m.K8sNodeCondition = &Metric{Enabled: true}
	},
	metricK8sPodStatusReason: func(m *K8sClusterMetrics) {
		initOptionalMetrics(m)
		m.K8sPodStatusReason = &Metric{Enabled: true}
	},
	metricK8sServiceEndpointCount: func(m *K8sClusterMetrics) {
		initOptionalMetrics(m)
		m.K8sServiceEndpointCount = &Metric{Enabled: true}
	},
	metricK8sServiceLBIngressCount: func(m *K8sClusterMetrics) {
		initOptionalMetrics(m)
		m.K8sServiceLoadBalancerIngressCount = &Metric{Enabled: true}
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
