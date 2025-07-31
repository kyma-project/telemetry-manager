# Runtime Input

To enable collection of runtime metrics, define a MetricPipeline that has the `runtime` section enabled as input:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
spec:
  input:
    runtime:
      enabled: true
  output:
    otlp:
      endpoint:
        value: https://backend.example.com:4317
```

## Resource Selection

By default, metrics for all resources (Pod, container, Node, Volume, DaemonSet, Deployment, StatefulSet, and Job) are collected.
To enable or disable the collection of metrics for a specific resource, use the `resources` section in the `runtime` input.

The following example collects only DaemonSet, Deployment, StatefulSet, and Job metrics:

  ```yaml
  apiVersion: telemetry.kyma-project.io/v1alpha1
  kind: MetricPipeline
  metadata:
    name: backend
  spec:
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
    output:
      otlp:
        endpoint:
          value: https://backend.example.com:4317
  ```

## Metrics

If Pod metrics are enabled, the following metrics are collected:

- From the [kubletstatsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/kubeletstatsreceiver):
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

If container metrics are enabled, the following metrics are collected:

- From the [kubletstatsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/kubeletstatsreceiver):
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

If Node metrics are enabled, the following metrics are collected:

- From the [kubletstatsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/kubeletstatsreceiver):
  - `k8s.node.cpu.usage`
  - `k8s.node.filesystem.available`
  - `k8s.node.filesystem.capacity`
  - `k8s.node.filesystem.usage`
  - `k8s.node.memory.available`
  - `k8s.node.memory.usage`
  - `k8s.node.memory.rss`
  - `k8s.node.memory.working_set`

If Volume metrics are enabled, the following metrics are collected:

- From the [kubletstatsreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/kubeletstatsreceiver):
  - `k8s.volume.available`
  - `k8s.volume.capacity`
  - `k8s.volume.inodes`
  - `k8s.volume.inodes.free`
  - `k8s.volume.inodes.used`

If Deployment metrics are enabled, the following metrics are collected:

- From the [k8sclusterreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sclusterreceiver):
  - `k8s.deployment.available`
  - `k8s.deployment.desired`

If DaemonSet metrics are enabled, the following metrics are collected:

- From the [k8sclusterreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sclusterreceiver):
  - `k8s.daemonset.current_scheduled_nodes`
  - `k8s.daemonset.desired_scheduled_nodes`
  - `k8s.daemonset.misscheduled_nodes`
  - `k8s.daemonset.ready_nodes`

If StatefulSet metrics are enabled, the following metrics are collected:

- From the [k8sclusterreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sclusterreceiver):
  - `k8s.statefulset.current_pods`
  - `k8s.statefulset.desired_pods`
  - `k8s.statefulset.ready_pods`
  - `k8s.statefulset.updated_pods`

If Job metrics are enabled, the following metrics are collected:

- From the [k8sclusterreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/k8sclusterreceiver):
  - `k8s.job.active_pods`
  - `k8s.job.desired_successful_pods`
  - `k8s.job.failed_pods`
  - `k8s.job.max_parallel_pods`
  - `k8s.job.successful_pods`

## Filters

To filter metrics by namespaces, define a MetricPipeline that has the `namespaces` section defined in one of the inputs. For example, you can specify the namespaces from which metrics are collected or the namespaces from which metrics are dropped. Learn more about the available [parameters and attributes](resources/05-metricpipeline.md).

The following example collects runtime metrics **only** from the `foo` and `bar` namespaces:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
spec:
  input:
    runtime:
      enabled: true
      namespaces:
        include:
          - foo
          - bar
  output:
    otlp:
      endpoint:
        value: https://backend.example.com:4317
```

The following example collects runtime metrics from all namespaces **except** the `foo` and `bar` namespaces:

```yaml
apiVersion: telemetry.kyma-project.io/v1alpha1
kind: MetricPipeline
metadata:
  name: backend
spec:
  input:
    runtime:
      enabled: true
      namespaces:
        exclude:
          - foo
          - bar
  output:
    otlp:
      endpoint:
        value: https://backend.example.com:4317
```
