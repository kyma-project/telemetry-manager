# Runtime Metrics

## Pod Metrics

If `pod` metrics are enabled, the following metrics are collected:

- From the [kubeletstatsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/kubeletstatsreceiver):
  - `k8s.pod.cpu.capacity`
  - `k8s.pod.cpu.usage`
  - `k8s.pod.filesystem.available`
  - `k8s.pod.filesystem.capacity`
  - `k8s.pod.filesystem.usage`
  - `k8s.pod.memory.available`
  - `k8s.pod.memory.major_page_faults`
  - `k8s.pod.memory.page_faults`
  - `k8s.pod.memory.rss`
  - `k8s.pod.memory.usage`
  - `k8s.pod.memory.working_set`
  - `k8s.pod.network.errors`
  - `k8s.pod.network.io`
- From the [k8sclusterreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sclusterreceiver):
  - `k8s.pod.phase`

## Container Metrics

If `container` metrics are enabled, the following metrics are collected:

- From the [kubeletstatsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/kubeletstatsreceiver):
  - `container.cpu.time`
  - `container.cpu.usage`
  - `container.filesystem.available`
  - `container.filesystem.capacity`
  - `container.filesystem.usage`
  - `container.memory.available`
  - `container.memory.major_page_faults`
  - `container.memory.page_faults`
  - `container.memory.rss`
  - `container.memory.usage`
  - `container.memory.working_set`
- From the [k8sclusterreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sclusterreceiver):
  - `k8s.container.cpu_request`
  - `k8s.container.cpu_limit`
  - `k8s.container.memory_request`
  - `k8s.container.memory_limit`
  - `k8s.container.restarts`

## Node Metrics

If `node` metrics are enabled, the following metrics are collected:

- From the [kubeletstatsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/kubeletstatsreceiver):
  - `k8s.node.cpu.usage`
  - `k8s.node.filesystem.available`
  - `k8s.node.filesystem.capacity`
  - `k8s.node.filesystem.usage`
  - `k8s.node.memory.available`
  - `k8s.node.memory.usage`
  - `k8s.node.memory.rss`
  - `k8s.node.memory.working_set`
  - `k8s.node.network.errors`,
  - `k8s.node.network.io`,

## Volume Metrics

If `volume` metrics are enabled, the following metrics are collected:

- From the [kubeletstatsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/kubeletstatsreceiver):
  - `k8s.volume.available`
  - `k8s.volume.capacity`
  - `k8s.volume.inodes`
  - `k8s.volume.inodes.free`
  - `k8s.volume.inodes.used`

## Deployment Metrics

If `deployment` metrics are enabled, the following metrics are collected:

- From the [k8sclusterreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sclusterreceiver):
  - `k8s.deployment.available`
  - `k8s.deployment.desired`

## DaemonSet Metrics

If `daemonset` metrics are enabled, the following metrics are collected:

- From the [k8sclusterreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sclusterreceiver):
  - `k8s.daemonset.current_scheduled_nodes`
  - `k8s.daemonset.desired_scheduled_nodes`
  - `k8s.daemonset.misscheduled_nodes`
  - `k8s.daemonset.ready_nodes`

## StatefulSet Metrics

If `statefulset` metrics are enabled, the following metrics are collected:

- From the [k8sclusterreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sclusterreceiver):
  - `k8s.statefulset.current_pods`
  - `k8s.statefulset.desired_pods`
  - `k8s.statefulset.ready_pods`
  - `k8s.statefulset.updated_pods`

## Job Metrics

If `job` metrics are enabled, the following metrics are collected:

- From the [k8sclusterreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sclusterreceiver):
  - `k8s.job.active_pods`
  - `k8s.job.desired_successful_pods`
  - `k8s.job.failed_pods`
  - `k8s.job.max_parallel_pods`
  - `k8s.job.successful_pods`

# Runtime Additional Metrics

The following metrics can be collected from the [kubeletstatsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/kubeletstatsreceiver):
- `container.cpu.time`
- `container.cpu.usage`
- `container.filesystem.available`
- `container.filesystem.capacity`
- `container.filesystem.usage`
- `container.memory.available`
- `container.memory.major_page_faults`
- `container.memory.page_faults`
- `container.memory.rss`
- `container.memory.usage`
- `container.memory.working_set`
- `k8s.node.cpu.time`
- `k8s.node.cpu.usage`
- `k8s.node.filesystem.available`
- `k8s.node.filesystem.capacity`
- `k8s.node.filesystem.usage`
- `k8s.node.memory.available`
- `k8s.node.memory.major_page_faults`
- `k8s.node.memory.page_faults`
- `k8s.node.memory.rss`
- `k8s.node.memory.usage`
- `k8s.node.memory.working_set`
- `k8s.node.network.errors`
- `k8s.node.network.io`
- `k8s.pod.cpu.time`
- `k8s.pod.cpu.usage`
- `k8s.pod.filesystem.available`
- `k8s.pod.filesystem.capacity`
- `k8s.pod.filesystem.usage`
- `k8s.pod.memory.available`
- `k8s.pod.memory.major_page_faults`
- `k8s.pod.memory.page_faults`
- `k8s.pod.memory.rss`
- `k8s.pod.memory.usage`
- `k8s.pod.memory.working_set`
- `k8s.pod.network.errors`
- `k8s.pod.network.io`
- `k8s.volume.available`
- `k8s.volume.capacity`
- `k8s.volume.inodes`
- `k8s.volume.inodes.free`
- `k8s.volume.inodes.used`
- `container.uptime`
- `k8s.container.cpu.node.utilization`
- `k8s.container.cpu_limit_utilization`
- `k8s.container.cpu_request_utilization`
- `k8s.container.memory.node.utilization`
- `k8s.container.memory_limit_utilization`
- `k8s.container.memory_request_utilization`
- `k8s.node.uptime`
- `k8s.pod.cpu.node.utilization`
- `k8s.pod.cpu_limit_utilization`
- `k8s.pod.cpu_request_utilization`
- `k8s.pod.memory.node.utilization`
- `k8s.pod.memory_limit_utilization`
- `k8s.pod.memory_request_utilization`
- `k8s.pod.uptime`
- `k8s.pod.volume.usage`

The following metrics can be collected from the [k8sclusterreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sclusterreceiver):
- `k8s.container.cpu_limit`
- `k8s.container.cpu_request`
- `k8s.container.ephemeralstorage_limit`
- `k8s.container.ephemeralstorage_request`
- `k8s.container.memory_limit`
- `k8s.container.memory_request`
- `k8s.container.ready`
- `k8s.container.restarts`
- `k8s.container.storage_limit`
- `k8s.container.storage_request`
- `k8s.container.status.reason`
- `k8s.container.status.state`
- `k8s.cronjob.active_jobs`
- `k8s.daemonset.current_scheduled_nodes`
- `k8s.daemonset.desired_scheduled_nodes`
- `k8s.daemonset.misscheduled_nodes`
- `k8s.daemonset.ready_nodes`
- `k8s.deployment.available`
- `k8s.deployment.desired`
- `k8s.hpa.current_replicas`
- `k8s.hpa.desired_replicas`
- `k8s.hpa.max_replicas`
- `k8s.hpa.min_replicas`
- `k8s.job.active_pods`
- `k8s.job.desired_successful_pods`
- `k8s.job.failed_pods`
- `k8s.job.max_parallel_pods`
- `k8s.job.successful_pods`
- `k8s.namespace.phase`
- `k8s.node.condition`
- `k8s.pod.phase`
- `k8s.pod.status_reason`
- `k8s.replicaset.available`
- `k8s.replicaset.desired`
- `k8s.replication_controller.available`
- `k8s.replication_controller.desired`
- `k8s.resource_quota.hard_limit`
- `k8s.resource_quota.used`
- `k8s.service.endpoint.count`
- `k8s.service.load_balancer.ingress.count`
- `k8s.statefulset.current_pods`
- `k8s.statefulset.desired_pods`
- `k8s.statefulset.ready_pods`
- `k8s.statefulset.updated_pods`
