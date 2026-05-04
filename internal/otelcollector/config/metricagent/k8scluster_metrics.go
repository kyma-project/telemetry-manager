package metricagent

// K8s cluster receiver metric name constants (excluding openshift specific metrics).
// Source: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/k8sclusterreceiver/documentation.md
const (
	// Default metrics
	metricK8sContainerCPULimit              = "k8s.container.cpu_limit"
	metricK8sContainerCPURequest            = "k8s.container.cpu_request"
	metricK8sContainerEphemeralStorageLimit = "k8s.container.ephemeralstorage_limit"
	metricK8sContainerEphemeralStorageReq   = "k8s.container.ephemeralstorage_request"
	metricK8sContainerMemoryLimit           = "k8s.container.memory_limit"
	metricK8sContainerMemoryRequest         = "k8s.container.memory_request"
	metricK8sContainerReady                 = "k8s.container.ready"
	metricK8sContainerRestarts              = "k8s.container.restarts"
	metricK8sContainerStorageLimit          = "k8s.container.storage_limit"
	metricK8sContainerStorageRequest        = "k8s.container.storage_request"
	metricK8sCronJobActiveJobs              = "k8s.cronjob.active_jobs"
	metricK8sDaemonSetCurrentScheduledNodes = "k8s.daemonset.current_scheduled_nodes"
	metricK8sDaemonSetDesiredScheduledNodes = "k8s.daemonset.desired_scheduled_nodes"
	metricK8sDaemonSetMisscheduledNodes     = "k8s.daemonset.misscheduled_nodes"
	metricK8sDaemonSetReadyNodes            = "k8s.daemonset.ready_nodes"
	metricK8sDeploymentAvailable            = "k8s.deployment.available"
	metricK8sDeploymentDesired              = "k8s.deployment.desired"
	metricK8sHPACurrentReplicas             = "k8s.hpa.current_replicas"
	metricK8sHPADesiredReplicas             = "k8s.hpa.desired_replicas"
	metricK8sHPAMaxReplicas                 = "k8s.hpa.max_replicas"
	metricK8sHPAMinReplicas                 = "k8s.hpa.min_replicas"
	metricK8sJobActivePods                  = "k8s.job.active_pods"
	metricK8sJobDesiredSuccessfulPods       = "k8s.job.desired_successful_pods"
	metricK8sJobFailedPods                  = "k8s.job.failed_pods"
	metricK8sJobMaxParallelPods             = "k8s.job.max_parallel_pods"
	metricK8sJobSuccessfulPods              = "k8s.job.successful_pods"
	metricK8sNamespacePhase                 = "k8s.namespace.phase"
	metricK8sPodPhase                       = "k8s.pod.phase"
	metricK8sReplicaSetAvailable            = "k8s.replicaset.available"
	metricK8sReplicaSetDesired              = "k8s.replicaset.desired"
	metricK8sReplicationControllerAvailable = "k8s.replication_controller.available"
	metricK8sReplicationControllerDesired   = "k8s.replication_controller.desired"
	metricK8sResourceQuotaHardLimit         = "k8s.resource_quota.hard_limit"
	metricK8sResourceQuotaUsed              = "k8s.resource_quota.used"
	metricK8sStatefulSetCurrentPods         = "k8s.statefulset.current_pods"
	metricK8sStatefulSetDesiredPods         = "k8s.statefulset.desired_pods"
	metricK8sStatefulSetReadyPods           = "k8s.statefulset.ready_pods"
	metricK8sStatefulSetUpdatedPods         = "k8s.statefulset.updated_pods"

	// Optional metrics
	metricK8sContainerStatusReason = "k8s.container.status.reason"
	metricK8sContainerStatusState  = "k8s.container.status.state"
	metricK8sNodeCondition         = "k8s.node.condition"
	metricK8sPVStatusPhase         = "k8s.persistentvolume.status.phase"
	metricK8sPVStorageCapacity     = "k8s.persistentvolume.storage.capacity"
	metricK8sPVCStatusPhase        = "k8s.persistentvolumeclaim.status.phase"
	metricK8sPVCStorageCapacity    = "k8s.persistentvolumeclaim.storage.capacity"
	metricK8sPVCStorageRequest     = "k8s.persistentvolumeclaim.storage.request"
	metricK8sPodStatusReason       = "k8s.pod.status_reason"
	metricK8sServiceEndpointCount  = "k8s.service.endpoint.count"
	metricK8sServiceLBIngressCount = "k8s.service.load_balancer.ingress.count"
)

// K8sClusterReceiverMetrics contains all metric names that can be emitted by the k8s_cluster receiver.
var K8sClusterReceiverMetrics = []string{
	// Default metrics
	metricK8sContainerCPULimit,
	metricK8sContainerCPURequest,
	metricK8sContainerEphemeralStorageLimit,
	metricK8sContainerEphemeralStorageReq,
	metricK8sContainerMemoryLimit,
	metricK8sContainerMemoryRequest,
	metricK8sContainerReady,
	metricK8sContainerRestarts,
	metricK8sContainerStorageLimit,
	metricK8sContainerStorageRequest,
	metricK8sCronJobActiveJobs,
	metricK8sDaemonSetCurrentScheduledNodes,
	metricK8sDaemonSetDesiredScheduledNodes,
	metricK8sDaemonSetMisscheduledNodes,
	metricK8sDaemonSetReadyNodes,
	metricK8sDeploymentAvailable,
	metricK8sDeploymentDesired,
	metricK8sHPACurrentReplicas,
	metricK8sHPADesiredReplicas,
	metricK8sHPAMaxReplicas,
	metricK8sHPAMinReplicas,
	metricK8sJobActivePods,
	metricK8sJobDesiredSuccessfulPods,
	metricK8sJobFailedPods,
	metricK8sJobMaxParallelPods,
	metricK8sJobSuccessfulPods,
	metricK8sNamespacePhase,
	metricK8sPodPhase,
	metricK8sReplicaSetAvailable,
	metricK8sReplicaSetDesired,
	metricK8sReplicationControllerAvailable,
	metricK8sReplicationControllerDesired,
	metricK8sResourceQuotaHardLimit,
	metricK8sResourceQuotaUsed,
	metricK8sStatefulSetCurrentPods,
	metricK8sStatefulSetDesiredPods,
	metricK8sStatefulSetReadyPods,
	metricK8sStatefulSetUpdatedPods,

	// Optional metrics
	metricK8sContainerStatusReason,
	metricK8sContainerStatusState,
	metricK8sNodeCondition,
	metricK8sPVStatusPhase,
	metricK8sPVStorageCapacity,
	metricK8sPVCStatusPhase,
	metricK8sPVCStorageCapacity,
	metricK8sPVCStorageRequest,
	metricK8sPodStatusReason,
	metricK8sServiceEndpointCount,
	metricK8sServiceLBIngressCount,
}
