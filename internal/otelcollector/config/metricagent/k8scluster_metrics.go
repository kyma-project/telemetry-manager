package metricagent

import "slices"

// K8s cluster receiver metric name constants (excluding openshift specific metrics).
const (
	// Upstream default metrics
	metricK8sContainerCPULimit                = "k8s.container.cpu_limit"
	metricK8sContainerCPURequest              = "k8s.container.cpu_request"
	metricK8sContainerEphemeralStorageLimit   = "k8s.container.ephemeralstorage_limit"
	metricK8sContainerEphemeralStorageRequest = "k8s.container.ephemeralstorage_request"
	metricK8sContainerMemoryLimit             = "k8s.container.memory_limit"
	metricK8sContainerMemoryRequest           = "k8s.container.memory_request"
	metricK8sContainerReady                   = "k8s.container.ready"
	metricK8sContainerRestarts                = "k8s.container.restarts"
	metricK8sContainerStorageLimit            = "k8s.container.storage_limit"
	metricK8sContainerStorageRequest          = "k8s.container.storage_request"
	metricK8sCronJobActiveJobs                = "k8s.cronjob.active_jobs"
	metricK8sDaemonSetCurrentScheduledNodes   = "k8s.daemonset.current_scheduled_nodes"
	metricK8sDaemonSetDesiredScheduledNodes   = "k8s.daemonset.desired_scheduled_nodes"
	metricK8sDaemonSetMisscheduledNodes       = "k8s.daemonset.misscheduled_nodes"
	metricK8sDaemonSetReadyNodes              = "k8s.daemonset.ready_nodes"
	metricK8sDeploymentAvailable              = "k8s.deployment.available"
	metricK8sDeploymentDesired                = "k8s.deployment.desired"
	metricK8sHPACurrentReplicas               = "k8s.hpa.current_replicas"
	metricK8sHPADesiredReplicas               = "k8s.hpa.desired_replicas"
	metricK8sHPAMaxReplicas                   = "k8s.hpa.max_replicas"
	metricK8sHPAMinReplicas                   = "k8s.hpa.min_replicas"
	metricK8sJobActivePods                    = "k8s.job.active_pods"
	metricK8sJobDesiredSuccessfulPods         = "k8s.job.desired_successful_pods"
	metricK8sJobFailedPods                    = "k8s.job.failed_pods"
	metricK8sJobMaxParallelPods               = "k8s.job.max_parallel_pods"
	metricK8sJobSuccessfulPods                = "k8s.job.successful_pods"
	metricK8sNamespacePhase                   = "k8s.namespace.phase"
	metricK8sPodPhase                         = "k8s.pod.phase"
	metricK8sReplicaSetAvailable              = "k8s.replicaset.available"
	metricK8sReplicaSetDesired                = "k8s.replicaset.desired"
	metricK8sReplicationControllerAvailable   = "k8s.replication_controller.available"
	metricK8sReplicationControllerDesired     = "k8s.replication_controller.desired"
	metricK8sResourceQuotaHardLimit           = "k8s.resource_quota.hard_limit"
	metricK8sResourceQuotaUsed                = "k8s.resource_quota.used"
	metricK8sStatefulSetCurrentPods           = "k8s.statefulset.current_pods"
	metricK8sStatefulSetDesiredPods           = "k8s.statefulset.desired_pods"
	metricK8sStatefulSetReadyPods             = "k8s.statefulset.ready_pods"
	metricK8sStatefulSetUpdatedPods           = "k8s.statefulset.updated_pods"

	// Upstream optional metrics
	metricK8sContainerStatusReason = "k8s.container.status.reason"
	metricK8sContainerStatusState  = "k8s.container.status.state"
	metricK8sNodeCondition         = "k8s.node.condition"
	metricK8sPodStatusReason       = "k8s.pod.status_reason"
	metricK8sServiceEndpointCount  = "k8s.service.endpoint.count"
	metricK8sServiceLBIngressCount = "k8s.service.load_balancer.ingress.count"
)

// k8sClusterReceiverPodMetrics contains metrics related to pod resources.
var k8sClusterReceiverPodMetrics = []string{
	metricK8sPodPhase,
}

// k8sClusterReceiverContainerMetrics contains curated list of metrics related to container resources.
var k8sClusterReceiverContainerMetrics = []string{
	metricK8sContainerCPURequest,
	metricK8sContainerCPULimit,
	metricK8sContainerMemoryRequest,
	metricK8sContainerMemoryLimit,
	metricK8sContainerRestarts,
}

// k8sClusterReceiverStatefulSetMetrics contains metrics related to statefulset resources.
var k8sClusterReceiverStatefulSetMetrics = []string{
	metricK8sStatefulSetCurrentPods,
	metricK8sStatefulSetDesiredPods,
	metricK8sStatefulSetReadyPods,
	metricK8sStatefulSetUpdatedPods,
}

// k8sClusterReceiverJobMetrics contains metrics related to job resources.
var k8sClusterReceiverJobMetrics = []string{
	metricK8sJobActivePods,
	metricK8sJobDesiredSuccessfulPods,
	metricK8sJobFailedPods,
	metricK8sJobMaxParallelPods,
	metricK8sJobSuccessfulPods,
}

// k8sClusterReceiverDeploymentMetrics contains metrics related to deployment resources.
var k8sClusterReceiverDeploymentMetrics = []string{
	metricK8sDeploymentAvailable,
	metricK8sDeploymentDesired,
}

// k8sClusterReceiverDaemonSetMetrics contains metrics related to daemonset resources.
var k8sClusterReceiverDaemonSetMetrics = []string{
	metricK8sDaemonSetCurrentScheduledNodes,
	metricK8sDaemonSetDesiredScheduledNodes,
	metricK8sDaemonSetMisscheduledNodes,
	metricK8sDaemonSetReadyNodes,
}

// k8sClusterReceiverExtraMetrics contains metrics that are disabled by default and optional metrics.
var k8sClusterReceiverExtraMetrics = []string{
	// Upstream default metrics that are disabled by default in the k8sCluster receiver
	metricK8sContainerStorageRequest,
	metricK8sContainerStorageLimit,
	metricK8sContainerEphemeralStorageRequest,
	metricK8sContainerEphemeralStorageLimit,
	metricK8sContainerReady,
	metricK8sNamespacePhase,
	metricK8sHPACurrentReplicas,
	metricK8sHPADesiredReplicas,
	metricK8sHPAMinReplicas,
	metricK8sHPAMaxReplicas,
	metricK8sReplicaSetAvailable,
	metricK8sReplicaSetDesired,
	metricK8sReplicationControllerAvailable,
	metricK8sReplicationControllerDesired,
	metricK8sResourceQuotaHardLimit,
	metricK8sResourceQuotaUsed,
	metricK8sCronJobActiveJobs,

	// Upstream optional metrics
	metricK8sContainerStatusReason,
	metricK8sContainerStatusState,
	metricK8sNodeCondition,
	metricK8sPodStatusReason,
	metricK8sServiceEndpointCount,
	metricK8sServiceLBIngressCount,
}

// K8sClusterReceiverMetrics contains all metric names that can be emitted by the k8sCluster receiver.
var K8sClusterReceiverMetrics = slices.Concat(
	k8sClusterReceiverPodMetrics,
	k8sClusterReceiverContainerMetrics,
	k8sClusterReceiverStatefulSetMetrics,
	k8sClusterReceiverJobMetrics,
	k8sClusterReceiverDeploymentMetrics,
	k8sClusterReceiverDaemonSetMetrics,
	k8sClusterReceiverExtraMetrics,
)
