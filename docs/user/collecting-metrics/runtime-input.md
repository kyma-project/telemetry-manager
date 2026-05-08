# Collect Runtime Metrics

To monitor the health and resource usage of your Kubernetes cluster, enable the **runtime** input in your MetricPipeline. This uses an agent on each node to gather metrics for resources like Pods, Nodes, and Deployments. You can choose the specific resources to monitor and control from which namespaces metrics are collected.

## Activate Runtime Metrics

By default, the **runtime** input is disabled. If you want to monitor your Kubernetes resources, enable the collection of runtime metrics:

```yaml
...
  input:
    runtime:
      enabled: true
```

With this, the Metric Agent starts collecting all runtime metrics from all resources (Pod, container, Node, Volume, DaemonSet, Deployment, StatefulSet, and Job).

> [!TIP]
> To select metrics from specific namespaces or to include system namespaces, see [Filter Metrics](../filter-and-process/filter-metrics.md).
> To change the scrape interval for runtime metrics, see [Configure Collection Interval](README.md#configure-collection-interval)).

## Select Resource Types

By default, metrics for all supported resource types are collected. To enable or disable the collection of metrics for a specific resource, use the **resources** section in the **runtime** input.

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

To learn which specific metrics are collected from which source (`kubeletstatsreceiver` or `k8sclusterreceiver`), see [Runtime Metrics](runtime-metrics.md#runtime-metrics).

## Collect Additional Metrics

The metric agent can also collect any metric which can be emitted from `kubeletstatsreceiver` or `k8sclusterreceiver`. To enable the collection of any of these metrics, add the desired metrics to the **additionalMetrics** list in the **runtime** input.

The following example collects `k8s.pod.memory_request_utilization` metric (from `kubeletstatsreceiver`) and `k8s.container.status.state` metric (from `k8sclusterreceiver`):

  ```yaml
  ...
    input:
      runtime:
        enabled: true
        additionalMetrics:
          - k8s.pod.memory_request_utilization
          - k8s.container.status.state
  ```

To learn which metrics can be added to the **additionalMetrics** list, see [Runtime Additional Metrics](runtime-metrics.md#runtime-additional-metrics)



Note that the **additionalMetrics** list overrules the **resources** section. For example, you can disable the **pod** metrics in the **runtime** input, but still add a specific pod metric in the **additionalMetrics** list and this metric will be collected.

The following example collects the `k8s.pod.memory_request_utilization` metric even though the **pod** metrics are disabled:

  ```yaml
  ...
    input:
      runtime:
        enabled: true
        resources:
          pod:
            enabled: false
        additionalMetrics:
          - k8s.pod.memory_request_utilization
  ```
