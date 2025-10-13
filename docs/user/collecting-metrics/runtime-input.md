# Collect Runtime Metrics

To monitor the health and resource usage of your Kubernetes cluster, enable the `runtime` input in your MetricPipeline. You can choose the specific resources you want to monitor, and you can control from which namespaces metrics are collected.

## Activate Runtime Metrics

By default, the `runtime` input is disabled. If you want to monitor your Kubernetes resources, enable the collection of runtime metrics:

```yaml
...
  input:
    runtime:
      enabled: true
```

With this, the metric agent starts collecting all runtime metrics from all resources (Pod, container, Node, Volume, DaemonSet, Deployment, StatefulSet, and Job).

## Select Resource Types

By default, metrics for all supported resource types are collected. To enable or disable the collection of metrics for a specific resource, use the `resources` section in the `runtime` input.

The following example collects only DaemonSet, Deployment, StatefulSet, and Job metrics:

  ```yaml
  ...
    input:
      runtime:
        enabled: true
        resources:
          pod:
            enabled: false
          container:
            enabled: false
          node:
            enabled: false
          volume:
            enabled: false
          daemonset:
            enabled: true
          deployment:
            enabled: true
          statefulset:
            enabled: true
          job:
            enabled: true
  ```

See a summary of the types of information you can gather for each resource:

|   Resource  |                            Metrics Collected                            |
|-------------|-------------------------------------------------------------------------|
| pod         | CPU, memory, filesystem, and network usage; current Pod phase           |
| container   | CPU/memory requests, limits, and usage; container restart count         |
| node        | Aggregated CPU, memory, filesystem, and network usage for the Node      |
| volume      | Filesystem capacity, usage, and inode statistics for persistent volumes |
| deployment  | Number of desired versus available replicas                             |
| daemonset   | Number of desired, current, and ready Nodes                             |
| statefulset | Number of desired, current, and ready Pods                              |
| job         | Counts of active, successful, and failed Pods                           |

To learn which specific metrics are collected from which source (`kubletstatsreceiver` or `k8sclusterreceiver`), see [Runtime Metrics](runtime-metrics.md).
